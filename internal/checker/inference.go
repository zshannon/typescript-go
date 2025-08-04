package checker

import (
	"slices"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/jsnum"
)

type InferenceKey struct {
	s TypeId
	t TypeId
}

type InferenceState struct {
	inferences        []*InferenceInfo
	originalSource    *Type
	originalTarget    *Type
	priority          InferencePriority
	inferencePriority InferencePriority
	contravariant     bool
	bivariant         bool
	expandingFlags    ExpandingFlags
	propagationType   *Type
	visited           map[InferenceKey]InferencePriority
	sourceStack       []*Type
	targetStack       []*Type
	next              *InferenceState
}

func (c *Checker) getInferenceState() *InferenceState {
	n := c.freeinferenceState
	if n == nil {
		n = &InferenceState{}
	}
	c.freeinferenceState = n.next
	return n
}

func (c *Checker) putInferenceState(n *InferenceState) {
	clear(n.visited)
	*n = InferenceState{
		inferences:  n.inferences[:0],
		visited:     n.visited,
		sourceStack: n.sourceStack[:0],
		targetStack: n.targetStack[:0],
		next:        c.freeinferenceState,
	}
	c.freeinferenceState = n
}

func (c *Checker) inferTypes(inferences []*InferenceInfo, originalSource *Type, originalTarget *Type, priority InferencePriority, contravariant bool) {
	n := c.getInferenceState()
	n.inferences = inferences
	n.originalSource = originalSource
	n.originalTarget = originalTarget
	n.priority = priority
	n.inferencePriority = InferencePriorityMaxValue
	n.contravariant = contravariant
	c.inferFromTypes(n, originalSource, originalTarget)
	c.putInferenceState(n)
}

func (c *Checker) inferFromTypes(n *InferenceState, source *Type, target *Type) {
	if !c.couldContainTypeVariables(target) || c.isNoInferType(target) {
		return
	}
	if source == c.wildcardType || source == c.blockedStringType {
		// We are inferring from an 'any' type. We want to infer this type for every type parameter
		// referenced in the target type, so we record it as the propagation type and infer from the
		// target to itself. Then, as we find candidates we substitute the propagation type.
		savePropagationType := n.propagationType
		n.propagationType = source
		c.inferFromTypes(n, target, target)
		n.propagationType = savePropagationType
		return
	}
	if source.alias != nil && target.alias != nil && source.alias.symbol == target.alias.symbol {
		if len(source.alias.typeArguments) != 0 || len(target.alias.typeArguments) != 0 {
			// Source and target are types originating in the same generic type alias declaration.
			// Simply infer from source type arguments to target type arguments, with defaults applied.
			params := c.typeAliasLinks.Get(source.alias.symbol).typeParameters
			minParams := c.getMinTypeArgumentCount(params)
			nodeIsInJsFile := ast.IsInJSFile(source.alias.symbol.ValueDeclaration)
			sourceTypes := c.fillMissingTypeArguments(source.alias.typeArguments, params, minParams, nodeIsInJsFile)
			targetTypes := c.fillMissingTypeArguments(target.alias.typeArguments, params, minParams, nodeIsInJsFile)
			c.inferFromTypeArguments(n, sourceTypes, targetTypes, c.getAliasVariances(source.alias.symbol))
		}
		// And if there weren't any type arguments, there's no reason to run inference as the types must be the same.
		return
	}
	if source == target && source.flags&TypeFlagsUnionOrIntersection != 0 {
		// When source and target are the same union or intersection type, just relate each constituent
		// type to itself.
		for _, t := range source.Types() {
			c.inferFromTypes(n, t, t)
		}
		return
	}
	if target.flags&TypeFlagsUnion != 0 && source.flags&TypeFlagsNever == 0 {
		// First, infer between identically matching source and target constituents and remove the
		// matching types.
		tempSources, tempTargets := c.inferFromMatchingTypes(n, source.Distributed(), target.Distributed(), (*Checker).isTypeOrBaseIdenticalTo)
		// Next, infer between closely matching source and target constituents and remove
		// the matching types. Types closely match when they are instantiations of the same
		// object type or instantiations of the same type alias.
		sources, targets := c.inferFromMatchingTypes(n, tempSources, tempTargets, (*Checker).isTypeCloselyMatchedBy)
		if len(targets) == 0 {
			return
		}
		target = c.getUnionType(targets)
		if len(sources) == 0 {
			// All source constituents have been matched and there is nothing further to infer from.
			// However, simply making no inferences is undesirable because it could ultimately mean
			// inferring a type parameter constraint. Instead, make a lower priority inference from
			// the full source to whatever remains in the target. For example, when inferring from
			// string to 'string | T', make a lower priority inference of string for T.
			c.inferWithPriority(n, source, target, InferencePriorityNakedTypeVariable)
			return
		}
		source = c.getUnionType(sources)
	} else if target.flags&TypeFlagsIntersection != 0 && !core.Every(target.Types(), c.isNonGenericObjectType) {
		// We reduce intersection types unless they're simple combinations of object types. For example,
		// when inferring from 'string[] & { extra: any }' to 'string[] & T' we want to remove string[] and
		// infer { extra: any } for T. But when inferring to 'string[] & Iterable<T>' we want to keep the
		// string[] on the source side and infer string for T.
		if source.flags&TypeFlagsUnion == 0 {
			var sourceTypes []*Type
			if source.flags&TypeFlagsIntersection != 0 {
				sourceTypes = source.Types()
			} else {
				sourceTypes = []*Type{source}
			}
			// Infer between identically matching source and target constituents and remove the matching types.
			sources, targets := c.inferFromMatchingTypes(n, sourceTypes, target.Types(), (*Checker).isTypeIdenticalTo)
			if len(sources) == 0 || len(targets) == 0 {
				return
			}
			source = c.getIntersectionType(sources)
			target = c.getIntersectionType(targets)
		}
	}
	if target.flags&(TypeFlagsIndexedAccess|TypeFlagsSubstitution) != 0 {
		if c.isNoInferType(target) {
			return
		}
		target = c.getActualTypeVariable(target)
	}
	if target.flags&TypeFlagsTypeVariable != 0 {
		// Skip inference if the source is "blocked", which is used by the language service to
		// prevent inference on nodes currently being edited.
		if c.isFromInferenceBlockedSource(source) {
			return
		}
		inference := getInferenceInfoForType(n, target)
		if inference != nil {
			// If target is a type parameter, make an inference, unless the source type contains
			// a "non-inferrable" type. Types with this flag set are markers used to prevent inference.
			//
			// For example:
			//     - anyFunctionType is a wildcard type that's used to avoid contextually typing functions;
			//       it's internal, so should not be exposed to the user by adding it as a candidate.
			//     - autoType (and autoArrayType) is a special "any" used in control flow; like anyFunctionType,
			//       it's internal and should not be observable.
			//     - silentNeverType is returned by getInferredType when instantiating a generic function for
			//       inference (and a type variable has no mapping).
			//
			// This flag is infectious; if we produce Box<never> (where never is silentNeverType), Box<never> is
			// also non-inferrable.
			//
			// As a special case, also ignore nonInferrableAnyType, which is a special form of the any type
			// used as a stand-in for binding elements when they are being inferred.
			if source.objectFlags&ObjectFlagsNonInferrableType != 0 || source == c.nonInferrableAnyType {
				return
			}
			if !inference.isFixed {
				candidate := core.OrElse(n.propagationType, source)
				if candidate == c.blockedStringType {
					return
				}
				if n.priority < inference.priority {
					inference.candidates = nil
					inference.contraCandidates = nil
					inference.topLevel = true
					inference.priority = n.priority
				}
				if n.priority == inference.priority {
					// Inferring A to [A[0]] is a zero information inference (it guarantees A becomes its constraint), but oft arises from generic argument list inferences
					// By discarding it early, we can allow more fruitful results to be used instead.
					if c.isTupleOfSelf(inference.typeParameter, candidate) {
						return
					}
					// We make contravariant inferences only if we are in a pure contravariant position,
					// i.e. only if we have not descended into a bivariant position.
					if n.contravariant && !n.bivariant {
						if !slices.Contains(inference.contraCandidates, candidate) {
							inference.contraCandidates = append(inference.contraCandidates, candidate)
							clearCachedInferences(n.inferences)
						}
					} else if !slices.Contains(inference.candidates, candidate) {
						inference.candidates = append(inference.candidates, candidate)
						clearCachedInferences(n.inferences)
					}
				}
				if n.priority&InferencePriorityReturnType == 0 && target.flags&TypeFlagsTypeParameter != 0 && inference.topLevel && !c.isTypeParameterAtTopLevel(n.originalTarget, target, 0) {
					inference.topLevel = false
					clearCachedInferences(n.inferences)
				}
			}
			n.inferencePriority = min(n.inferencePriority, n.priority)
			return
		}
		// Infer to the simplified version of an indexed access, if possible, to (hopefully) expose more bare type parameters to the inference engine
		simplified := c.getSimplifiedType(target, false /*writing*/)
		if simplified != target {
			c.inferFromTypes(n, source, simplified)
		} else if target.flags&TypeFlagsIndexedAccess != 0 {
			indexType := c.getSimplifiedType(target.AsIndexedAccessType().indexType, false /*writing*/)
			// Generally simplifications of instantiable indexes are avoided to keep relationship checking correct, however if our target is an access, we can consider
			// that key of that access to be "instantiated", since we're looking to find the infernce goal in any way we can.
			if indexType.flags&TypeFlagsInstantiable != 0 {
				simplified := c.distributeIndexOverObjectType(c.getSimplifiedType(target.AsIndexedAccessType().objectType, false /*writing*/), indexType, false /*writing*/)
				if simplified != nil && simplified != target {
					c.inferFromTypes(n, source, simplified)
				}
			}
		}
	}
	switch {
	case source.objectFlags&ObjectFlagsReference != 0 && target.objectFlags&ObjectFlagsReference != 0 && (source.AsTypeReference().target == target.AsTypeReference().target || c.isArrayType(source) && c.isArrayType(target)) && !(source.AsTypeReference().node != nil && target.AsTypeReference().node != nil):
		// If source and target are references to the same generic type, infer from type arguments
		c.inferFromTypeArguments(n, c.getTypeArguments(source), c.getTypeArguments(target), c.getVariances(source.AsTypeReference().target))
	case source.flags&TypeFlagsIndex != 0 && target.flags&TypeFlagsIndex != 0:
		c.inferFromContravariantTypes(n, source.AsIndexType().target, target.AsIndexType().target)
	case (isLiteralType(source) || source.flags&TypeFlagsString != 0) && target.flags&TypeFlagsIndex != 0:
		empty := c.createEmptyObjectTypeFromStringLiteral(source)
		c.inferFromContravariantTypesWithPriority(n, empty, target.AsIndexType().target, InferencePriorityLiteralKeyof)
	case source.flags&TypeFlagsIndexedAccess != 0 && target.flags&TypeFlagsIndexedAccess != 0:
		c.inferFromTypes(n, source.AsIndexedAccessType().objectType, target.AsIndexedAccessType().objectType)
		c.inferFromTypes(n, source.AsIndexedAccessType().indexType, target.AsIndexedAccessType().indexType)
	case source.flags&TypeFlagsStringMapping != 0 && target.flags&TypeFlagsStringMapping != 0:
		if source.symbol == target.symbol {
			c.inferFromTypes(n, source.AsStringMappingType().target, target.AsStringMappingType().target)
		}
	case source.flags&TypeFlagsSubstitution != 0:
		c.inferFromTypes(n, source.AsSubstitutionType().baseType, target)
		// Make substitute inference at a lower priority
		c.inferWithPriority(n, c.getSubstitutionIntersection(source), target, InferencePrioritySubstituteSource)
	case target.flags&TypeFlagsConditional != 0:
		c.invokeOnce(n, source, target, (*Checker).inferToConditionalType)
	case target.flags&TypeFlagsUnionOrIntersection != 0:
		c.inferToMultipleTypes(n, source, target.Types(), target.flags)
	case source.flags&TypeFlagsUnion != 0:
		// Source is a union or intersection type, infer from each constituent type
		for _, sourceType := range source.Types() {
			c.inferFromTypes(n, sourceType, target)
		}
	case target.flags&TypeFlagsTemplateLiteral != 0:
		c.inferToTemplateLiteralType(n, source, target.AsTemplateLiteralType())
	default:
		source = c.getReducedType(source)
		if c.isGenericMappedType(source) && c.isGenericMappedType(target) {
			c.invokeOnce(n, source, target, (*Checker).inferFromGenericMappedTypes)
		}
		if !(n.priority&InferencePriorityNoConstraints != 0 && source.flags&(TypeFlagsIntersection|TypeFlagsInstantiable) != 0) {
			apparentSource := c.getApparentType(source)
			// getApparentType can return _any_ type, since an indexed access or conditional may simplify to any other type.
			// If that occurs and it doesn't simplify to an object or intersection, we'll need to restart `inferFromTypes`
			// with the simplified source.
			if apparentSource != source && apparentSource.flags&(TypeFlagsObject|TypeFlagsIntersection) == 0 {
				c.inferFromTypes(n, apparentSource, target)
				return
			}
			source = apparentSource
		}
		if source.flags&(TypeFlagsObject|TypeFlagsIntersection) != 0 {
			c.invokeOnce(n, source, target, (*Checker).inferFromObjectTypes)
		}
	}
}

func (c *Checker) inferFromTypeArguments(n *InferenceState, sourceTypes []*Type, targetTypes []*Type, variances []VarianceFlags) {
	for i := range min(len(sourceTypes), len(targetTypes)) {
		if i < len(variances) && variances[i]&VarianceFlagsVarianceMask == VarianceFlagsContravariant {
			c.inferFromContravariantTypes(n, sourceTypes[i], targetTypes[i])
		} else {
			c.inferFromTypes(n, sourceTypes[i], targetTypes[i])
		}
	}
}

func (c *Checker) inferWithPriority(n *InferenceState, source *Type, target *Type, newPriority InferencePriority) {
	savePriority := n.priority
	n.priority |= newPriority
	c.inferFromTypes(n, source, target)
	n.priority = savePriority
}

func (c *Checker) inferFromContravariantTypesWithPriority(n *InferenceState, source *Type, target *Type, newPriority InferencePriority) {
	savePriority := n.priority
	n.priority |= newPriority
	c.inferFromContravariantTypes(n, source, target)
	n.priority = savePriority
}

func (c *Checker) inferFromContravariantTypes(n *InferenceState, source *Type, target *Type) {
	n.contravariant = !n.contravariant
	c.inferFromTypes(n, source, target)
	n.contravariant = !n.contravariant
}

func (c *Checker) inferFromContravariantTypesIfStrictFunctionTypes(n *InferenceState, source *Type, target *Type) {
	if c.strictFunctionTypes || n.priority&InferencePriorityAlwaysStrict != 0 {
		c.inferFromContravariantTypes(n, source, target)
	} else {
		c.inferFromTypes(n, source, target)
	}
}

// Ensure an inference action is performed only once for the given source and target types.
// This includes two things:
// Avoiding inferring between the same pair of source and target types,
// and avoiding circularly inferring between source and target types.
// For an example of the last, consider if we are inferring between source type
// `type Deep<T> = { next: Deep<Deep<T>> }` and target type `type Loop<U> = { next: Loop<U> }`.
// We would then infer between the types of the `next` property: `Deep<Deep<T>>` = `{ next: Deep<Deep<Deep<T>>> }` and `Loop<U>` = `{ next: Loop<U> }`.
// We will then infer again between the types of the `next` property:
// `Deep<Deep<Deep<T>>>` and `Loop<U>`, and so on, such that we would be forever inferring
// between instantiations of the same types `Deep` and `Loop`.
// In particular, we would be inferring from increasingly deep instantiations of `Deep` to `Loop`,
// such that we would go on inferring forever, even though we would never infer
// between the same pair of types.
func (c *Checker) invokeOnce(n *InferenceState, source *Type, target *Type, action func(c *Checker, n *InferenceState, source *Type, target *Type)) {
	key := InferenceKey{s: source.id, t: target.id}
	if status, ok := n.visited[key]; ok {
		n.inferencePriority = min(n.inferencePriority, status)
		return
	}
	if n.visited == nil {
		n.visited = make(map[InferenceKey]InferencePriority)
	}
	n.visited[key] = InferencePriorityCircularity
	saveInferencePriority := n.inferencePriority
	n.inferencePriority = InferencePriorityMaxValue
	// We stop inferring and report a circularity if we encounter duplicate recursion identities on both
	// the source side and the target side.
	saveExpandingFlags := n.expandingFlags
	n.sourceStack = append(n.sourceStack, source)
	n.targetStack = append(n.targetStack, target)
	if c.isDeeplyNestedType(source, n.sourceStack, 2) {
		n.expandingFlags |= ExpandingFlagsSource
	}
	if c.isDeeplyNestedType(target, n.targetStack, 2) {
		n.expandingFlags |= ExpandingFlagsTarget
	}
	if n.expandingFlags != ExpandingFlagsBoth {
		action(c, n, source, target)
	} else {
		n.inferencePriority = InferencePriorityCircularity
	}
	n.targetStack = n.targetStack[:len(n.targetStack)-1]
	n.sourceStack = n.sourceStack[:len(n.sourceStack)-1]
	n.expandingFlags = saveExpandingFlags
	n.visited[key] = n.inferencePriority
	n.inferencePriority = min(n.inferencePriority, saveInferencePriority)
}

func (c *Checker) inferFromMatchingTypes(n *InferenceState, sources []*Type, targets []*Type, matches func(c *Checker, s *Type, t *Type) bool) ([]*Type, []*Type) {
	var matchedSources []*Type
	var matchedTargets []*Type
	for _, t := range targets {
		for _, s := range sources {
			if matches(c, s, t) {
				c.inferFromTypes(n, s, t)
				matchedSources = core.AppendIfUnique(matchedSources, s)
				matchedTargets = core.AppendIfUnique(matchedTargets, t)
			}
		}
	}
	if len(matchedSources) != 0 {
		sources = core.Filter(sources, func(t *Type) bool { return !slices.Contains(matchedSources, t) })
	}
	if len(matchedTargets) != 0 {
		targets = core.Filter(targets, func(t *Type) bool { return !slices.Contains(matchedTargets, t) })
	}
	return sources, targets
}

func (c *Checker) inferToMultipleTypes(n *InferenceState, source *Type, targets []*Type, targetFlags TypeFlags) {
	typeVariableCount := 0
	if targetFlags&TypeFlagsUnion != 0 {
		var nakedTypeVariable *Type
		sources := source.Distributed()
		matched := make([]bool, len(sources))
		inferenceCircularity := false
		// First infer to types that are not naked type variables. For each source type we
		// track whether inferences were made from that particular type to some target with
		// equal priority (i.e. of equal quality) to what we would infer for a naked type
		// parameter.
		for _, t := range targets {
			if getInferenceInfoForType(n, t) != nil {
				nakedTypeVariable = t
				typeVariableCount++
			} else {
				for i := range sources {
					saveInferencePriority := n.inferencePriority
					n.inferencePriority = InferencePriorityMaxValue
					c.inferFromTypes(n, sources[i], t)
					if n.inferencePriority == n.priority {
						matched[i] = true
					}
					inferenceCircularity = inferenceCircularity || n.inferencePriority == InferencePriorityCircularity
					n.inferencePriority = min(n.inferencePriority, saveInferencePriority)
				}
			}
		}
		if typeVariableCount == 0 {
			// If every target is an intersection of types containing a single naked type variable,
			// make a lower priority inference to that type variable. This handles inferring from
			// 'A | B' to 'T & (X | Y)' where we want to infer 'A | B' for T.
			intersectionTypeVariable := getSingleTypeVariableFromIntersectionTypes(n, targets)
			if intersectionTypeVariable != nil {
				c.inferWithPriority(n, source, intersectionTypeVariable, InferencePriorityNakedTypeVariable)
			}
			return
		}
		// If the target has a single naked type variable and no inference circularities were
		// encountered above (meaning we explored the types fully), create a union of the source
		// types from which no inferences have been made so far and infer from that union to the
		// naked type variable.
		if typeVariableCount == 1 && !inferenceCircularity {
			var unmatched []*Type
			for i, s := range sources {
				if !matched[i] {
					unmatched = append(unmatched, s)
				}
			}
			if len(unmatched) != 0 {
				c.inferFromTypes(n, c.getUnionType(unmatched), nakedTypeVariable)
				return
			}
		}
	} else {
		// We infer from types that are not naked type variables first so that inferences we
		// make from nested naked type variables and given slightly higher priority by virtue
		// of being first in the candidates array.
		for _, t := range targets {
			if getInferenceInfoForType(n, t) != nil {
				typeVariableCount++
			} else {
				c.inferFromTypes(n, source, t)
			}
		}
	}
	// Inferences directly to naked type variables are given lower priority as they are
	// less specific. For example, when inferring from Promise<string> to T | Promise<T>,
	// we want to infer string for T, not Promise<string> | string. For intersection types
	// we only infer to single naked type variables.
	if targetFlags&TypeFlagsIntersection != 0 && typeVariableCount == 1 || targetFlags&TypeFlagsIntersection == 0 && typeVariableCount > 0 {
		for _, t := range targets {
			if getInferenceInfoForType(n, t) != nil {
				c.inferWithPriority(n, source, t, InferencePriorityNakedTypeVariable)
			}
		}
	}
}

func getSingleTypeVariableFromIntersectionTypes(n *InferenceState, types []*Type) *Type {
	var typeVariable *Type
	for _, t := range types {
		if t.flags&TypeFlagsIntersection == 0 {
			return nil
		}
		v := core.Find(t.Types(), func(t *Type) bool { return getInferenceInfoForType(n, t) != nil })
		if v == nil || typeVariable != nil && v != typeVariable {
			return nil
		}
		typeVariable = v
	}
	return typeVariable
}

func (c *Checker) inferToMultipleTypesWithPriority(n *InferenceState, source *Type, targets []*Type, targetFlags TypeFlags, newPriority InferencePriority) {
	savePriority := n.priority
	n.priority |= newPriority
	c.inferToMultipleTypes(n, source, targets, targetFlags)
	n.priority = savePriority
}

func (c *Checker) inferToConditionalType(n *InferenceState, source *Type, target *Type) {
	if source.flags&TypeFlagsConditional != 0 {
		c.inferFromTypes(n, source.AsConditionalType().checkType, target.AsConditionalType().checkType)
		c.inferFromTypes(n, source.AsConditionalType().extendsType, target.AsConditionalType().extendsType)
		c.inferFromTypes(n, c.getTrueTypeFromConditionalType(source), c.getTrueTypeFromConditionalType(target))
		c.inferFromTypes(n, c.getFalseTypeFromConditionalType(source), c.getFalseTypeFromConditionalType(target))
	} else {
		targetTypes := []*Type{c.getTrueTypeFromConditionalType(target), c.getFalseTypeFromConditionalType(target)}
		c.inferToMultipleTypesWithPriority(n, source, targetTypes, target.flags, core.IfElse(n.contravariant, InferencePriorityContravariantConditional, 0))
	}
}

func (c *Checker) inferToTemplateLiteralType(n *InferenceState, source *Type, target *TemplateLiteralType) {
	matches := c.inferTypesFromTemplateLiteralType(source, target)
	types := target.types
	// When the target template literal contains only placeholders (meaning that inference is intended to extract
	// single characters and remainder strings) and inference fails to produce matches, we want to infer 'never' for
	// each placeholder such that instantiation with the inferred value(s) produces 'never', a type for which an
	// assignment check will fail. If we make no inferences, we'll likely end up with the constraint 'string' which,
	// upon instantiation, would collapse all the placeholders to just 'string', and an assignment check might
	// succeed. That would be a pointless and confusing outcome.
	if len(matches) != 0 || core.Every(target.texts, func(s string) bool { return s == "" }) {
		for i, target := range types {
			var source *Type
			if len(matches) != 0 {
				source = matches[i]
			} else {
				source = c.neverType
			}
			// If we are inferring from a string literal type to a type variable whose constraint includes one of the
			// allowed template literal placeholder types, infer from a literal type corresponding to the constraint.
			if source.flags&TypeFlagsStringLiteral != 0 && target.flags&TypeFlagsTypeVariable != 0 {
				if inferenceContext := getInferenceInfoForType(n, target); inferenceContext != nil {
					if constraint := c.getBaseConstraintOfType(inferenceContext.typeParameter); constraint != nil && !IsTypeAny(constraint) {
						allTypeFlags := TypeFlagsNone
						for _, t := range constraint.Distributed() {
							allTypeFlags |= t.flags
						}
						// If the constraint contains `string`, we don't need to look for a more preferred type
						if allTypeFlags&TypeFlagsString == 0 {
							str := getStringLiteralValue(source)
							// If the type contains `number` or a number literal and the string isn't a valid number, exclude numbers
							if allTypeFlags&TypeFlagsNumberLike != 0 && !isValidNumberString(str, true /*roundTripOnly*/) {
								allTypeFlags &^= TypeFlagsNumberLike
							}
							// If the type contains `bigint` or a bigint literal and the string isn't a valid bigint, exclude bigints
							if allTypeFlags&TypeFlagsBigIntLike != 0 && !isValidBigIntString(str, true /*roundTripOnly*/) {
								allTypeFlags &^= TypeFlagsBigIntLike
							}
							choose := func(left *Type, right *Type) *Type {
								switch {
								case right.flags&allTypeFlags == 0:
									return left
								case left.flags&TypeFlagsString != 0:
									return left
								case right.flags&TypeFlagsString != 0:
									return source
								case left.flags&TypeFlagsTemplateLiteral != 0:
									return left
								case right.flags&TypeFlagsTemplateLiteral != 0 && c.isTypeMatchedByTemplateLiteralType(source, right.AsTemplateLiteralType()):
									return source
								case left.flags&TypeFlagsStringMapping != 0:
									return left
								case right.flags&TypeFlagsStringMapping != 0 && str == applyStringMapping(right.symbol, str):
									return source
								case left.flags&TypeFlagsStringLiteral != 0:
									return left
								case right.flags&TypeFlagsStringLiteral != 0 && getStringLiteralValue(right) == str:
									return right
								case left.flags&TypeFlagsNumber != 0:
									return left
								case right.flags&TypeFlagsNumber != 0:
									return c.getNumberLiteralType(jsnum.FromString(str))
								case left.flags&TypeFlagsEnum != 0:
									return left
								case right.flags&TypeFlagsEnum != 0:
									return c.getNumberLiteralType(jsnum.FromString(str))
								case left.flags&TypeFlagsNumberLiteral != 0:
									return left
								case right.flags&TypeFlagsNumberLiteral != 0 && getNumberLiteralValue(right) == jsnum.FromString(str):
									return right
								case left.flags&TypeFlagsBigInt != 0:
									return left
								case right.flags&TypeFlagsBigInt != 0:
									return c.parseBigIntLiteralType(str)
								case left.flags&TypeFlagsBigIntLiteral != 0:
									return left
								case right.flags&TypeFlagsBigIntLiteral != 0 && pseudoBigIntToString(getBigIntLiteralValue(right)) == str:
									return right
								case left.flags&TypeFlagsBoolean != 0:
									return left
								case right.flags&TypeFlagsBoolean != 0:
									switch {
									case str == "true":
										return c.trueType
									case str == "false":
										return c.falseType
									default:
										return c.booleanType
									}
								case left.flags&TypeFlagsBooleanLiteral != 0:
									return left
								case right.flags&TypeFlagsBooleanLiteral != 0 && core.IfElse(getBooleanLiteralValue(right), "true", "false") == str:
									return right
								case left.flags&TypeFlagsUndefined != 0:
									return left
								case right.flags&TypeFlagsUndefined != 0 && right.AsIntrinsicType().intrinsicName == str:
									return right
								case left.flags&TypeFlagsNull != 0:
									return left
								case right.flags&TypeFlagsNull != 0 && right.AsIntrinsicType().intrinsicName == str:
									return right
								default:
									return left
								}
							}
							matchingType := c.neverType
							for _, t := range constraint.Distributed() {
								matchingType = choose(matchingType, t)
							}
							if matchingType.flags&TypeFlagsNever == 0 {
								c.inferFromTypes(n, matchingType, target)
								continue
							}
						}
					}
				}
			}
			c.inferFromTypes(n, source, target)
		}
	}
}

func (c *Checker) inferFromGenericMappedTypes(n *InferenceState, source *Type, target *Type) {
	// The source and target types are generic types { [P in S]: X } and { [P in T]: Y }, so we infer
	// from S to T and from X to Y.
	c.inferFromTypes(n, c.getConstraintTypeFromMappedType(source), c.getConstraintTypeFromMappedType(target))
	c.inferFromTypes(n, c.getTemplateTypeFromMappedType(source), c.getTemplateTypeFromMappedType(target))
	sourceNameType := c.getNameTypeFromMappedType(source)
	targetNameType := c.getNameTypeFromMappedType(target)
	if sourceNameType != nil && targetNameType != nil {
		c.inferFromTypes(n, sourceNameType, targetNameType)
	}
}

func (c *Checker) inferFromObjectTypes(n *InferenceState, source *Type, target *Type) {
	if source.objectFlags&ObjectFlagsReference != 0 && target.objectFlags&ObjectFlagsReference != 0 && (source.Target() == target.Target() || c.isArrayType(source) && c.isArrayType(target)) {
		// If source and target are references to the same generic type, infer from type arguments
		c.inferFromTypeArguments(n, c.getTypeArguments(source), c.getTypeArguments(target), c.getVariances(source.Target()))
		return
	}
	if c.isGenericMappedType(source) && c.isGenericMappedType(target) {
		c.inferFromGenericMappedTypes(n, source, target)
	}
	if target.objectFlags&ObjectFlagsMapped != 0 && target.AsMappedType().declaration.NameType == nil {
		constraintType := c.getConstraintTypeFromMappedType(target)
		if c.inferToMappedType(n, source, target, constraintType) {
			return
		}
	}
	// Infer from the members of source and target only if the two types are possibly related
	if c.typesDefinitelyUnrelated(source, target) {
		return
	}
	if c.isArrayOrTupleType(source) {
		if isTupleType(target) {
			sourceArity := c.getTypeReferenceArity(source)
			targetArity := c.getTypeReferenceArity(target)
			elementTypes := c.getTypeArguments(target)
			elementInfos := target.TargetTupleType().elementInfos
			// When source and target are tuple types with the same structure (fixed, variadic, and rest are matched
			// to the same kind in each position), simply infer between the element types.
			if isTupleType(source) && c.isTupleTypeStructureMatching(source, target) {
				for i := range targetArity {
					c.inferFromTypes(n, c.getTypeArguments(source)[i], elementTypes[i])
				}
				return
			}
			startLength := 0
			endLength := 0
			if isTupleType(source) {
				startLength = min(source.TargetTupleType().fixedLength, target.TargetTupleType().fixedLength)
				if target.TargetTupleType().combinedFlags&ElementFlagsVariable != 0 {
					endLength = min(getEndElementCount(source.TargetTupleType(), ElementFlagsFixed), getEndElementCount(target.TargetTupleType(), ElementFlagsFixed))
				}
			}
			// Infer between starting fixed elements.
			for i := range startLength {
				c.inferFromTypes(n, c.getTypeArguments(source)[i], elementTypes[i])
			}
			if !isTupleType(source) || sourceArity-startLength-endLength == 1 && source.TargetTupleType().elementInfos[startLength].flags&ElementFlagsRest != 0 {
				// Single rest element remains in source, infer from that to every element in target
				restType := c.getTypeArguments(source)[startLength]
				for i := startLength; i < targetArity-endLength; i++ {
					t := restType
					if elementInfos[i].flags&ElementFlagsVariadic != 0 {
						t = c.createArrayType(t)
					}
					c.inferFromTypes(n, t, elementTypes[i])
				}
			} else {
				middleLength := targetArity - startLength - endLength
				if middleLength == 2 {
					if elementInfos[startLength].flags&elementInfos[startLength+1].flags&ElementFlagsVariadic != 0 {
						// Middle of target is [...T, ...U] and source is tuple type
						targetInfo := getInferenceInfoForType(n, elementTypes[startLength])
						if targetInfo != nil && targetInfo.impliedArity >= 0 {
							// Infer slices from source based on implied arity of T.
							c.inferFromTypes(n, c.sliceTupleType(source, startLength, endLength+sourceArity-targetInfo.impliedArity), elementTypes[startLength])
							c.inferFromTypes(n, c.sliceTupleType(source, startLength+targetInfo.impliedArity, endLength), elementTypes[startLength+1])
						}
					} else if elementInfos[startLength].flags&ElementFlagsVariadic != 0 && elementInfos[startLength+1].flags&ElementFlagsRest != 0 {
						// Middle of target is [...T, ...rest] and source is tuple type
						// if T is constrained by a fixed-size tuple we might be able to use its arity to infer T
						if info := getInferenceInfoForType(n, elementTypes[startLength]); info != nil {
							constraint := c.getBaseConstraintOfType(info.typeParameter)
							if constraint != nil && isTupleType(constraint) && constraint.TargetTupleType().combinedFlags&ElementFlagsVariable == 0 {
								impliedArity := constraint.TargetTupleType().fixedLength
								c.inferFromTypes(n, c.sliceTupleType(source, startLength, sourceArity-(startLength+impliedArity)), elementTypes[startLength])
								c.inferFromTypes(n, c.getElementTypeOfSliceOfTupleType(source, startLength+impliedArity, endLength, false, false), elementTypes[startLength+1])
							}
						}
					} else if elementInfos[startLength].flags&ElementFlagsRest != 0 && elementInfos[startLength+1].flags&ElementFlagsVariadic != 0 {
						// Middle of target is [...rest, ...T] and source is tuple type
						// if T is constrained by a fixed-size tuple we might be able to use its arity to infer T
						if info := getInferenceInfoForType(n, elementTypes[startLength+1]); info != nil {
							constraint := c.getBaseConstraintOfType(info.typeParameter)
							if constraint != nil && isTupleType(constraint) && constraint.TargetTupleType().combinedFlags&ElementFlagsVariable == 0 {
								impliedArity := constraint.TargetTupleType().fixedLength
								endIndex := sourceArity - getEndElementCount(target.TargetTupleType(), ElementFlagsFixed)
								startIndex := endIndex - impliedArity
								trailingSlice := c.createTupleTypeEx(c.getTypeArguments(source)[startIndex:endIndex], source.TargetTupleType().elementInfos[startIndex:endIndex], false /*readonly*/)
								c.inferFromTypes(n, c.getElementTypeOfSliceOfTupleType(source, startLength, endLength+impliedArity, false, false), elementTypes[startLength])
								c.inferFromTypes(n, trailingSlice, elementTypes[startLength+1])
							}
						}
					}
				} else if middleLength == 1 && elementInfos[startLength].flags&ElementFlagsVariadic != 0 {
					// Middle of target is exactly one variadic element. Infer the slice between the fixed parts in the source.
					// If target ends in optional element(s), make a lower priority a speculative inference.
					priority := core.IfElse(elementInfos[targetArity-1].flags&ElementFlagsOptional != 0, InferencePrioritySpeculativeTuple, 0)
					sourceSlice := c.sliceTupleType(source, startLength, endLength)
					c.inferWithPriority(n, sourceSlice, elementTypes[startLength], priority)
				} else if middleLength == 1 && elementInfos[startLength].flags&ElementFlagsRest != 0 {
					// Middle of target is exactly one rest element. If middle of source is not empty, infer union of middle element types.
					restType := c.getElementTypeOfSliceOfTupleType(source, startLength, endLength, false, false)
					if restType != nil {
						c.inferFromTypes(n, restType, elementTypes[startLength])
					}
				}
			}
			// Infer between ending fixed elements
			for i := range endLength {
				c.inferFromTypes(n, c.getTypeArguments(source)[sourceArity-i-1], elementTypes[targetArity-i-1])
			}
			return
		}
		if c.isArrayType(target) {
			c.inferFromIndexTypes(n, source, target)
			return
		}
	}
	c.inferFromProperties(n, source, target)
	c.inferFromSignatures(n, source, target, SignatureKindCall)
	c.inferFromSignatures(n, source, target, SignatureKindConstruct)
	c.inferFromIndexTypes(n, source, target)
}

func (c *Checker) inferFromProperties(n *InferenceState, source *Type, target *Type) {
	properties := c.getPropertiesOfObjectType(target)
	for _, targetProp := range properties {
		sourceProp := c.getPropertyOfType(source, targetProp.Name)
		if sourceProp != nil && !core.Some(sourceProp.Declarations, c.isSkipDirectInferenceNode) {
			c.inferFromTypes(n, c.removeMissingType(c.getTypeOfSymbol(sourceProp), sourceProp.Flags&ast.SymbolFlagsOptional != 0), c.removeMissingType(c.getTypeOfSymbol(targetProp), targetProp.Flags&ast.SymbolFlagsOptional != 0))
		}
	}
}

func (c *Checker) inferFromSignatures(n *InferenceState, source *Type, target *Type, kind SignatureKind) {
	sourceSignatures := c.getSignaturesOfType(source, kind)
	sourceLen := len(sourceSignatures)
	if sourceLen > 0 {
		// We match source and target signatures from the bottom up, and if the source has fewer signatures
		// than the target, we infer from the first source signature to the excess target signatures.
		targetSignatures := c.getSignaturesOfType(target, kind)
		targetLen := len(targetSignatures)
		for i := range targetLen {
			sourceIndex := max(sourceLen-targetLen+i, 0)
			c.inferFromSignature(n, c.getBaseSignature(sourceSignatures[sourceIndex]), c.getErasedSignature(targetSignatures[i]))
		}
	}
}

func (c *Checker) inferFromSignature(n *InferenceState, source *Signature, target *Signature) {
	if source.flags&SignatureFlagsIsNonInferrable == 0 {
		saveBivariant := n.bivariant
		kind := ast.KindUnknown
		if target.declaration != nil {
			kind = target.declaration.Kind
		}
		// Once we descend into a bivariant signature we remain bivariant for all nested inferences
		n.bivariant = n.bivariant || kind == ast.KindMethodDeclaration || kind == ast.KindMethodSignature || kind == ast.KindConstructor
		c.applyToParameterTypes(source, target, func(s, t *Type) { c.inferFromContravariantTypesIfStrictFunctionTypes(n, s, t) })
		n.bivariant = saveBivariant
	}
	c.applyToReturnTypes(source, target, func(s, t *Type) { c.inferFromTypes(n, s, t) })
}

func (c *Checker) applyToParameterTypes(source *Signature, target *Signature, callback func(s *Type, t *Type)) {
	sourceCount := c.getParameterCount(source)
	targetCount := c.getParameterCount(target)
	sourceRestType := c.getEffectiveRestType(source)
	targetRestType := c.getEffectiveRestType(target)
	targetNonRestCount := targetCount
	if targetRestType != nil {
		targetNonRestCount--
	}
	paramCount := targetNonRestCount
	if sourceRestType == nil {
		paramCount = min(sourceCount, targetNonRestCount)
	}
	sourceThisType := c.getThisTypeOfSignature(source)
	if sourceThisType != nil {
		targetThisType := c.getThisTypeOfSignature(target)
		if targetThisType != nil {
			callback(sourceThisType, targetThisType)
		}
	}
	for i := range paramCount {
		callback(c.getTypeAtPosition(source, i), c.getTypeAtPosition(target, i))
	}
	if targetRestType != nil {
		callback(c.getRestTypeAtPosition(source, paramCount, c.isConstTypeVariable(targetRestType, 0) && !someType(targetRestType, c.isMutableArrayLikeType) /*readonly*/), targetRestType)
	}
}

func (c *Checker) applyToReturnTypes(source *Signature, target *Signature, callback func(s *Type, t *Type)) {
	targetTypePredicate := c.getTypePredicateOfSignature(target)
	if targetTypePredicate != nil {
		sourceTypePredicate := c.getTypePredicateOfSignature(source)
		if sourceTypePredicate != nil && c.typePredicateKindsMatch(sourceTypePredicate, targetTypePredicate) && sourceTypePredicate.t != nil && targetTypePredicate.t != nil {
			callback(sourceTypePredicate.t, targetTypePredicate.t)
			return
		}
	}
	targetReturnType := c.getReturnTypeOfSignature(target)
	if c.couldContainTypeVariables(targetReturnType) {
		callback(c.getReturnTypeOfSignature(source), targetReturnType)
	}
}

func (c *Checker) inferFromIndexTypes(n *InferenceState, source *Type, target *Type) {
	// Inferences across mapped type index signatures are pretty much the same a inferences to homomorphic variables
	priority := InferencePriorityNone
	if source.objectFlags&target.objectFlags&ObjectFlagsMapped != 0 {
		priority = InferencePriorityHomomorphicMappedType
	}
	indexInfos := c.getIndexInfosOfType(target)
	if c.isObjectTypeWithInferableIndex(source) {
		for _, targetInfo := range indexInfos {
			var propTypes []*Type
			for _, prop := range c.getPropertiesOfType(source) {
				if c.isApplicableIndexType(c.getLiteralTypeFromProperty(prop, TypeFlagsStringOrNumberLiteralOrUnique, false), targetInfo.keyType) {
					propType := c.getTypeOfSymbol(prop)
					if prop.Flags&ast.SymbolFlagsOptional != 0 {
						propType = c.removeMissingOrUndefinedType(propType)
					}
					propTypes = append(propTypes, propType)
				}
			}
			for _, info := range c.getIndexInfosOfType(source) {
				if c.isApplicableIndexType(info.keyType, targetInfo.keyType) {
					propTypes = append(propTypes, info.valueType)
				}
			}
			if len(propTypes) != 0 {
				c.inferWithPriority(n, c.getUnionType(propTypes), targetInfo.valueType, priority)
			}
		}
	}
	for _, targetInfo := range indexInfos {
		sourceInfo := c.getApplicableIndexInfo(source, targetInfo.keyType)
		if sourceInfo != nil {
			c.inferWithPriority(n, sourceInfo.valueType, targetInfo.valueType, priority)
		}
	}
}

func (c *Checker) inferToMappedType(n *InferenceState, source *Type, target *Type, constraintType *Type) bool {
	if constraintType.flags&TypeFlagsUnion != 0 || constraintType.flags&TypeFlagsIntersection != 0 {
		result := false
		for _, t := range constraintType.Types() {
			result = core.OrElse(c.inferToMappedType(n, source, target, t), result)
		}
		return result
	}
	if constraintType.flags&TypeFlagsIndex != 0 {
		// We're inferring from some source type S to a homomorphic mapped type { [P in keyof T]: X },
		// where T is a type variable. Use inferTypeForHomomorphicMappedType to infer a suitable source
		// type and then make a secondary inference from that type to T. We make a secondary inference
		// such that direct inferences to T get priority over inferences to Partial<T>, for example.
		inference := getInferenceInfoForType(n, constraintType.AsIndexType().target)
		if inference != nil && !inference.isFixed && !c.isFromInferenceBlockedSource(source) {
			inferredType := c.inferTypeForHomomorphicMappedType(source, target, constraintType)
			if inferredType != nil {
				// We assign a lower priority to inferences made from types containing non-inferrable
				// types because we may only have a partial result (i.e. we may have failed to make
				// reverse inferences for some properties).
				c.inferWithPriority(n, inferredType, inference.typeParameter, core.IfElse(source.objectFlags&ObjectFlagsNonInferrableType != 0, InferencePriorityPartialHomomorphicMappedType, InferencePriorityHomomorphicMappedType))
			}
		}
		return true
	}
	if constraintType.flags&TypeFlagsTypeParameter != 0 {
		// We're inferring from some source type S to a mapped type { [P in K]: X }, where K is a type
		// parameter. First infer from 'keyof S' to K.
		c.inferWithPriority(n, c.getIndexTypeEx(source, core.IfElse(c.patternForType[source] != nil, IndexFlagsNoIndexSignatures, IndexFlagsNone)), constraintType, InferencePriorityMappedTypeConstraint)
		// If K is constrained to a type C, also infer to C. Thus, for a mapped type { [P in K]: X },
		// where K extends keyof T, we make the same inferences as for a homomorphic mapped type
		// { [P in keyof T]: X }. This enables us to make meaningful inferences when the target is a
		// Pick<T, K>.
		extendedConstraint := c.getConstraintOfType(constraintType)
		if extendedConstraint != nil && c.inferToMappedType(n, source, target, extendedConstraint) {
			return true
		}
		// If no inferences can be made to K's constraint, infer from a union of the property types
		// in the source to the template type X.
		propTypes := core.Map(c.getPropertiesOfType(source), c.getTypeOfSymbol)
		indexTypes := core.Map(c.getIndexInfosOfType(source), func(info *IndexInfo) *Type {
			if info != c.enumNumberIndexInfo {
				return info.valueType
			}
			return c.neverType
		})
		c.inferFromTypes(n, c.getUnionType(core.Concatenate(propTypes, indexTypes)), c.getTemplateTypeFromMappedType(target))
		return true
	}
	return false
}

// Infer a suitable input type for a homomorphic mapped type { [P in keyof T]: X }. We construct
// an object type with the same set of properties as the source type, where the type of each
// property is computed by inferring from the source property type to X for the type
// variable T[P] (i.e. we treat the type T[P] as the type variable we're inferring for).
func (c *Checker) inferTypeForHomomorphicMappedType(source *Type, target *Type, constraint *Type) *Type {
	key := ReverseMappedTypeKey{sourceId: source.id, targetId: target.id, constraintId: constraint.id}
	if cached := c.reverseHomomorphicMappedCache[key]; cached != nil {
		return cached
	}
	t := c.createReverseMappedType(source, target, constraint)
	c.reverseHomomorphicMappedCache[key] = t
	return t
}

func (c *Checker) createReverseMappedType(source *Type, target *Type, constraint *Type) *Type {
	// We consider a source type reverse mappable if it has a string index signature or if
	// it has one or more properties and is of a partially inferable type.
	if !(c.getIndexInfoOfType(source, c.stringType) != nil || len(c.getPropertiesOfType(source)) != 0 && c.isPartiallyInferableType(source)) {
		return nil
	}
	// For arrays and tuples we infer new arrays and tuples where the reverse mapping has been
	// applied to the element type(s).
	if c.isArrayType(source) {
		elementType := c.inferReverseMappedType(c.getTypeArguments(source)[0], target, constraint)
		if elementType == nil {
			return nil
		}
		return c.createArrayTypeEx(elementType, c.isReadonlyArrayType(source))
	}
	if isTupleType(source) {
		elementTypes := core.Map(c.getElementTypes(source), func(t *Type) *Type {
			return c.inferReverseMappedType(t, target, constraint)
		})
		if !core.Every(elementTypes, func(t *Type) bool { return t != nil }) {
			return nil
		}
		elementInfos := source.TargetTupleType().elementInfos
		if getMappedTypeModifiers(target)&MappedTypeModifiersIncludeOptional != 0 {
			elementInfos = core.SameMap(elementInfos, func(info TupleElementInfo) TupleElementInfo {
				if info.flags&ElementFlagsOptional != 0 {
					return TupleElementInfo{flags: ElementFlagsRequired, labeledDeclaration: info.labeledDeclaration}
				}
				return info
			})
		}
		return c.createTupleTypeEx(elementTypes, elementInfos, source.TargetTupleType().readonly)
	}
	// For all other object types we infer a new object type where the reverse mapping has been
	// applied to the type of each property.
	reversed := c.newObjectType(ObjectFlagsReverseMapped|ObjectFlagsAnonymous, nil /*symbol*/)
	reversed.AsReverseMappedType().source = source
	reversed.AsReverseMappedType().mappedType = target
	reversed.AsReverseMappedType().constraintType = constraint
	return reversed
}

// We consider a type to be partially inferable if it isn't marked non-inferable or if it is
// an object literal type with at least one property of an inferable type. For example, an object
// literal { a: 123, b: x => true } is marked non-inferable because it contains a context sensitive
// arrow function, but is considered partially inferable because property 'a' has an inferable type.
func (c *Checker) isPartiallyInferableType(t *Type) bool {
	return t.objectFlags&ObjectFlagsNonInferrableType == 0 || isObjectLiteralType(t) && core.Some(c.getPropertiesOfType(t), func(prop *ast.Symbol) bool {
		return c.isPartiallyInferableType(c.getTypeOfSymbol(prop))
	}) || isTupleType(t) && core.Some(c.getElementTypes(t), c.isPartiallyInferableType)
}

func (c *Checker) inferReverseMappedType(source *Type, target *Type, constraint *Type) *Type {
	key := ReverseMappedTypeKey{sourceId: source.id, targetId: target.id, constraintId: constraint.id}
	if cached, ok := c.reverseMappedCache[key]; ok {
		return core.OrElse(cached, c.unknownType)
	}
	c.reverseMappedSourceStack = append(c.reverseMappedSourceStack, source)
	c.reverseMappedTargetStack = append(c.reverseMappedTargetStack, target)
	saveExpandingFlags := c.reverseExpandingFlags
	if c.isDeeplyNestedType(source, c.reverseMappedSourceStack, 2) {
		c.reverseExpandingFlags |= ExpandingFlagsSource
	}
	if c.isDeeplyNestedType(target, c.reverseMappedTargetStack, 2) {
		c.reverseExpandingFlags |= ExpandingFlagsTarget
	}
	var t *Type
	if c.reverseExpandingFlags != ExpandingFlagsBoth {
		t = c.inferReverseMappedTypeWorker(source, target, constraint)
	}
	c.reverseMappedSourceStack = c.reverseMappedSourceStack[:len(c.reverseMappedSourceStack)-1]
	c.reverseMappedTargetStack = c.reverseMappedTargetStack[:len(c.reverseMappedTargetStack)-1]
	c.reverseExpandingFlags = saveExpandingFlags
	c.reverseMappedCache[key] = t
	return t
}

func (c *Checker) inferReverseMappedTypeWorker(source *Type, target *Type, constraint *Type) *Type {
	typeParameter := c.getIndexedAccessType(constraint.AsIndexType().target, c.getTypeParameterFromMappedType(target))
	templateType := c.getTemplateTypeFromMappedType(target)
	inference := newInferenceInfo(typeParameter)
	c.inferTypes([]*InferenceInfo{inference}, source, templateType, InferencePriorityNone, false)
	return core.OrElse(c.getTypeFromInference(inference), c.unknownType)
}

func (c *Checker) resolveReverseMappedTypeMembers(t *Type) {
	r := t.AsReverseMappedType()
	indexInfo := c.getIndexInfoOfType(r.source, c.stringType)
	modifiers := getMappedTypeModifiers(r.mappedType)
	readonlyMask := modifiers&MappedTypeModifiersIncludeReadonly == 0
	optionalMask := core.IfElse(modifiers&MappedTypeModifiersIncludeOptional != 0, 0, ast.SymbolFlagsOptional)
	var indexInfos []*IndexInfo
	if indexInfo != nil {
		indexInfos = []*IndexInfo{c.newIndexInfo(c.stringType, core.OrElse(c.inferReverseMappedType(indexInfo.valueType, r.mappedType, r.constraintType), c.unknownType), readonlyMask && indexInfo.isReadonly, nil)}
	}
	members := make(ast.SymbolTable)
	limitedConstraint := c.getLimitedConstraint(t)
	for _, prop := range c.getPropertiesOfType(r.source) {
		// In case of a reverse mapped type with an intersection constraint, if we were able to
		// extract the filtering type literals we skip those properties that are not assignable to them,
		// because the extra properties wouldn't get through the application of the mapped type anyway
		if limitedConstraint != nil {
			propertyNameType := c.getLiteralTypeFromProperty(prop, TypeFlagsStringOrNumberLiteralOrUnique, false)
			if !c.isTypeAssignableTo(propertyNameType, limitedConstraint) {
				continue
			}
		}
		checkFlags := ast.CheckFlagsReverseMapped | core.IfElse(readonlyMask && c.isReadonlySymbol(prop), ast.CheckFlagsReadonly, 0)
		inferredProp := c.newSymbolEx(ast.SymbolFlagsProperty|prop.Flags&optionalMask, prop.Name, checkFlags)
		inferredProp.Declarations = prop.Declarations
		c.valueSymbolLinks.Get(inferredProp).nameType = c.valueSymbolLinks.Get(prop).nameType
		links := c.ReverseMappedSymbolLinks.Get(inferredProp)
		links.propertyType = c.getTypeOfSymbol(prop)
		constraintTarget := r.constraintType.AsIndexType().target
		if constraintTarget.flags&TypeFlagsIndexedAccess != 0 && constraintTarget.AsIndexedAccessType().objectType.flags&TypeFlagsTypeParameter != 0 && constraintTarget.AsIndexedAccessType().indexType.flags&TypeFlagsTypeParameter != 0 {
			// A reverse mapping of `{[K in keyof T[K_1]]: T[K_1]}` is the same as that of `{[K in keyof T]: T}`, since all we care about is
			// inferring to the "type parameter" (or indexed access) shared by the constraint and template. So, to reduce the number of
			// type identities produced, we simplify such indexed access occurrences
			newTypeParam := constraintTarget.AsIndexedAccessType().objectType
			newMappedType := c.replaceIndexedAccess(r.mappedType, constraintTarget, newTypeParam)
			links.mappedType = newMappedType
			links.constraintType = c.getIndexType(newTypeParam)
		} else {
			links.mappedType = r.mappedType
			links.constraintType = r.constraintType
		}
		members[prop.Name] = inferredProp
	}
	c.setStructuredTypeMembers(t, members, nil, nil, indexInfos)
}

func (c *Checker) getTypeOfReverseMappedSymbol(symbol *ast.Symbol) *Type {
	links := c.valueSymbolLinks.Get(symbol)
	if links.resolvedType == nil {
		reverseLinks := c.ReverseMappedSymbolLinks.Get(symbol)
		links.resolvedType = core.OrElse(c.inferReverseMappedType(reverseLinks.propertyType, reverseLinks.mappedType, reverseLinks.constraintType), c.unknownType)
	}
	return links.resolvedType
}

// If the original mapped type had an intersection constraint we extract its components,
// and we make an attempt to do so even if the intersection has been reduced to a union.
// This entire process allows us to possibly retrieve the filtering type literals.
// e.g. { [K in keyof U & ("a" | "b") ] } -> "a" | "b"
func (c *Checker) getLimitedConstraint(t *Type) *Type {
	constraint := c.getConstraintTypeFromMappedType(t.AsReverseMappedType().mappedType)
	if !(constraint.flags&TypeFlagsUnion != 0 || constraint.flags&TypeFlagsIntersection != 0) {
		return nil
	}
	origin := constraint
	if constraint.flags&TypeFlagsUnion != 0 {
		origin = constraint.AsUnionType().origin
	}
	if origin == nil || origin.flags&TypeFlagsIntersection == 0 {
		return nil
	}
	constraintType := t.AsReverseMappedType().constraintType
	limitedConstraint := c.getIntersectionType(core.Filter(origin.Types(), func(t *Type) bool { return t != constraintType }))
	if limitedConstraint != c.neverType {
		return limitedConstraint
	}
	return nil
}

func (c *Checker) replaceIndexedAccess(instantiable *Type, t *Type, replacement *Type) *Type {
	// map type.indexType to 0
	// map type.objectType to `[TReplacement]`
	// thus making the indexed access `[TReplacement][0]` or `TReplacement`
	return c.instantiateType(instantiable, newTypeMapper([]*Type{t.AsIndexedAccessType().indexType, t.AsIndexedAccessType().objectType}, []*Type{c.getNumberLiteralType(0), c.createTupleType([]*Type{replacement})}))
}

func (c *Checker) typesDefinitelyUnrelated(source *Type, target *Type) bool {
	// Two tuple types with incompatible arities are definitely unrelated.
	// Two object types that each have a property that is unmatched in the other are definitely unrelated.
	if isTupleType(source) && isTupleType(target) {
		return tupleTypesDefinitelyUnrelated(source, target)
	}
	return c.getUnmatchedProperty(source, target, false /*requireOptionalProperties*/, true /*matchDiscriminantProperties*/) != nil &&
		c.getUnmatchedProperty(target, source, false /*requireOptionalProperties*/, false /*matchDiscriminantProperties*/) != nil
}

func tupleTypesDefinitelyUnrelated(source *Type, target *Type) bool {
	s := source.TargetTupleType()
	t := target.TargetTupleType()
	return t.combinedFlags&ElementFlagsVariadic == 0 && t.minLength > s.minLength ||
		t.combinedFlags&ElementFlagsVariable == 0 && (s.combinedFlags&ElementFlagsVariable != 0 || t.fixedLength < s.fixedLength)
}

func (c *Checker) isTupleTypeStructureMatching(t1 *Type, t2 *Type) bool {
	if c.getTypeReferenceArity(t1) != c.getTypeReferenceArity(t2) {
		return false
	}
	for i, e := range t1.TargetTupleType().elementInfos {
		if e.flags&ElementFlagsVariable != t2.TargetTupleType().elementInfos[i].flags&ElementFlagsVariable {
			return false
		}
	}
	return true
}

func (c *Checker) isTypeOrBaseIdenticalTo(s *Type, t *Type) bool {
	if t == c.missingType {
		return s == t
	}
	return c.isTypeIdenticalTo(s, t) ||
		t.flags&TypeFlagsString != 0 && s.flags&TypeFlagsStringLiteral != 0 ||
		t.flags&TypeFlagsNumber != 0 && s.flags&TypeFlagsNumberLiteral != 0
}

func (c *Checker) isTypeCloselyMatchedBy(s *Type, t *Type) bool {
	return s.flags&TypeFlagsObject != 0 && t.flags&TypeFlagsObject != 0 && s.symbol != nil && s.symbol == t.symbol ||
		s.alias != nil && t.alias != nil && len(s.alias.typeArguments) != 0 && s.alias.symbol == t.alias.symbol
}

// Create an object with properties named in the string literal type. Every property has type `any`.
func (c *Checker) createEmptyObjectTypeFromStringLiteral(t *Type) *Type {
	members := make(ast.SymbolTable)
	for _, t := range t.Distributed() {
		if t.flags&TypeFlagsStringLiteral == 0 {
			continue
		}
		name := getStringLiteralValue(t)
		literalProp := c.newSymbol(ast.SymbolFlagsProperty, name)
		c.valueSymbolLinks.Get(literalProp).resolvedType = c.anyType
		if t.symbol != nil {
			literalProp.Declarations = t.symbol.Declarations
			literalProp.ValueDeclaration = t.symbol.ValueDeclaration
		}
		members[name] = literalProp
	}
	var indexInfos []*IndexInfo
	if t.flags&TypeFlagsString != 0 {
		indexInfos = []*IndexInfo{c.newIndexInfo(c.stringType, c.emptyObjectType, false /*isReadonly*/, nil)}
	}
	return c.newAnonymousType(nil, members, nil, nil, indexInfos)
}

func (c *Checker) newInferenceContext(typeParameters []*Type, signature *Signature, flags InferenceFlags, compareTypes TypeComparer) *InferenceContext {
	if compareTypes == nil {
		compareTypes = c.compareTypesAssignable
	}
	return c.newInferenceContextWorker(core.Map(typeParameters, newInferenceInfo), signature, flags, compareTypes)
}

func (c *Checker) cloneInferenceContext(n *InferenceContext, extraFlags InferenceFlags) *InferenceContext {
	if n == nil {
		return nil
	}
	return c.newInferenceContextWorker(core.Map(n.inferences, cloneInferenceInfo), n.signature, n.flags|extraFlags, n.compareTypes)
}

func (c *Checker) cloneInferredPartOfContext(n *InferenceContext) *InferenceContext {
	inferences := core.Filter(n.inferences, hasInferenceCandidates)
	if len(inferences) == 0 {
		return nil
	}
	return c.newInferenceContextWorker(core.Map(inferences, cloneInferenceInfo), n.signature, n.flags, n.compareTypes)
}

func (c *Checker) newInferenceContextWorker(inferences []*InferenceInfo, signature *Signature, flags InferenceFlags, compareTypes TypeComparer) *InferenceContext {
	n := &InferenceContext{
		inferences:   inferences,
		signature:    signature,
		flags:        flags,
		compareTypes: compareTypes,
	}
	n.mapper = c.newInferenceTypeMapper(n, true /*fixing*/)
	n.nonFixingMapper = c.newInferenceTypeMapper(n, false /*fixing*/)
	return n
}

func (c *Checker) addIntraExpressionInferenceSite(n *InferenceContext, node *ast.Node, t *Type) {
	n.intraExpressionInferenceSites = append(n.intraExpressionInferenceSites, IntraExpressionInferenceSite{node: node, t: t})
}

// We collect intra-expression inference sites within object and array literals to handle cases where
// inferred types flow between context sensitive element expressions. For example:
//
//	declare function foo<T>(arg: [(n: number) => T, (x: T) => void]): void;
//	foo([_a => 0, n => n.toFixed()]);
//
// Above, both arrow functions in the tuple argument are context sensitive, thus both are omitted from the
// pass that collects inferences from the non-context sensitive parts of the arguments. In the subsequent
// pass where nothing is omitted, we need to commit to an inference for T in order to contextually type the
// parameter in the second arrow function, but we want to first infer from the return type of the first
// arrow function. This happens automatically when the arrow functions are discrete arguments (because we
// infer from each argument before processing the next), but when the arrow functions are elements of an
// object or array literal, we need to perform intra-expression inferences early.
func (c *Checker) inferFromIntraExpressionSites(n *InferenceContext) {
	for _, site := range n.intraExpressionInferenceSites {
		var contextualType *Type
		if ast.IsMethodDeclaration(site.node) {
			contextualType = c.getContextualTypeForObjectLiteralMethod(site.node, ContextFlagsNoConstraints)
		} else {
			contextualType = c.getContextualType(site.node, ContextFlagsNoConstraints)
		}
		if contextualType != nil {
			c.inferTypes(n.inferences, site.t, contextualType, InferencePriorityNone, false)
		}
	}
	n.intraExpressionInferenceSites = nil
}

func (c *Checker) getInferredType(n *InferenceContext, index int) *Type {
	inference := n.inferences[index]
	if inference.inferredType == nil {
		if inference.typeParameter == c.errorType {
			return inference.typeParameter
		}
		var inferredType *Type
		var fallbackType *Type
		if n.signature != nil {
			var inferredCovariantType *Type
			if len(inference.candidates) != 0 {
				inferredCovariantType = c.getCovariantInference(inference, n.signature)
			}
			var inferredContravariantType *Type
			if len(inference.contraCandidates) != 0 {
				inferredContravariantType = c.getContravariantInference(inference)
			}
			if inferredCovariantType != nil || inferredContravariantType != nil {
				// If we have both co- and contra-variant inferences, we prefer the co-variant inference if it is not 'never',
				// all co-variant inferences are assignable to it (i.e. it isn't one of a conflicting set of candidates), it is
				// assignable to some contra-variant inference, and no other type parameter is constrained to this type parameter
				// and has inferences that would conflict. Otherwise, we prefer the contra-variant inference.
				// Similarly ignore co-variant `any` inference when both are available as almost everything is assignable to it
				// and it would spoil the overall inference.
				preferCovariantType := inferredCovariantType != nil && (inferredContravariantType == nil ||
					inferredCovariantType.flags&(TypeFlagsNever|TypeFlagsAny) == 0 &&
						core.Some(inference.contraCandidates, func(t *Type) bool { return c.isTypeAssignableTo(inferredCovariantType, t) }) &&
						core.Every(n.inferences, func(other *InferenceInfo) bool {
							return other != inference && c.getConstraintOfTypeParameter(other.typeParameter) != inference.typeParameter ||
								core.Every(other.candidates, func(t *Type) bool { return c.isTypeAssignableTo(t, inferredCovariantType) })
						}))
				if preferCovariantType {
					inferredType = inferredCovariantType
					fallbackType = inferredContravariantType
				} else {
					inferredType = inferredContravariantType
					fallbackType = inferredCovariantType
				}
			} else if n.flags&InferenceFlagsNoDefault != 0 {
				// We use silentNeverType as the wildcard that signals no inferences.
				inferredType = c.silentNeverType
			} else {
				// Infer either the default or the empty object type when no inferences were
				// made. It is important to remember that in this case, inference still
				// succeeds, meaning there is no error for not having inference candidates. An
				// inference error only occurs when there are *conflicting* candidates, i.e.
				// candidates with no common supertype.
				defaultType := c.getDefaultFromTypeParameter(inference.typeParameter)
				if defaultType != nil {
					// Instantiate the default type. Any forward reference to a type
					// parameter should be instantiated to the empty object type.
					inferredType = c.instantiateType(defaultType, mergeTypeMappers(c.newBackreferenceMapper(n, index), n.nonFixingMapper))
				}
			}
		} else {
			inferredType = c.getTypeFromInference(inference)
		}
		inference.inferredType = inferredType
		if inference.inferredType == nil {
			inference.inferredType = core.IfElse(n.flags&InferenceFlagsAnyDefault != 0, c.anyType, c.unknownType)
		}
		constraint := c.getConstraintOfTypeParameter(inference.typeParameter)
		if constraint != nil {
			instantiatedConstraint := c.instantiateType(constraint, n.nonFixingMapper)
			if inferredType != nil {
				constraintWithThis := c.getTypeWithThisArgument(instantiatedConstraint, inferredType, false)
				if n.compareTypes(inferredType, constraintWithThis, false) == TernaryFalse {
					var filteredByConstraint *Type
					if inference.priority == InferencePriorityReturnType {
						// If we have a pure return type inference, we may succeed by removing constituents of the inferred type
						// that aren't assignable to the constraint type (pure return type inferences are speculation anyway).
						filteredByConstraint = c.mapType(inferredType, func(t *Type) *Type {
							return core.IfElse(n.compareTypes(t, constraintWithThis, false) != TernaryFalse, t, c.neverType)
						})
					}
					inferredType = core.IfElse(filteredByConstraint != nil && filteredByConstraint.flags&TypeFlagsNever == 0, filteredByConstraint, nil)
				}
			}
			if inferredType == nil {
				// If the fallback type satisfies the constraint, we pick it. Otherwise, we pick the constraint.
				inferredType = core.IfElse(fallbackType != nil && n.compareTypes(fallbackType, c.getTypeWithThisArgument(instantiatedConstraint, fallbackType, false), false) != TernaryFalse, fallbackType, instantiatedConstraint)
			}
			inference.inferredType = inferredType
		}
		c.clearActiveMapperCaches()
	}
	return inference.inferredType
}

func (c *Checker) getInferredTypes(n *InferenceContext) []*Type {
	result := make([]*Type, len(n.inferences))
	for i := range n.inferences {
		result[i] = c.getInferredType(n, i)
	}
	return result
}

func (c *Checker) getMapperFromContext(n *InferenceContext) *TypeMapper {
	if n == nil {
		return nil
	}
	return n.mapper
}

func (c *Checker) getCovariantInference(inference *InferenceInfo, signature *Signature) *Type {
	// Extract all object and array literal types and replace them with a single widened and normalized type.
	candidates := c.unionObjectAndArrayLiteralCandidates(inference.candidates)
	// We widen inferred literal types if
	// all inferences were made to top-level occurrences of the type parameter, and
	// the type parameter has no constraint or its constraint includes no primitive or literal types, and
	// the type parameter was fixed during inference or does not occur at top-level in the return type.
	primitiveConstraint := c.hasPrimitiveConstraint(inference.typeParameter) || c.isConstTypeVariable(inference.typeParameter, 0)
	widenLiteralTypes := !primitiveConstraint && inference.topLevel && (inference.isFixed || !c.isTypeParameterAtTopLevelInReturnType(signature, inference.typeParameter))
	var baseCandidates []*Type
	switch {
	case primitiveConstraint:
		baseCandidates = core.SameMap(candidates, c.getRegularTypeOfLiteralType)
	case widenLiteralTypes:
		baseCandidates = core.SameMap(candidates, c.getWidenedLiteralType)
	default:
		baseCandidates = candidates
	}
	// If all inferences were made from a position that implies a combined result, infer a union type.
	// Otherwise, infer a common supertype.
	var unwidenedType *Type
	if inference.priority&InferencePriorityPriorityImpliesCombination != 0 {
		unwidenedType = c.getUnionTypeEx(baseCandidates, UnionReductionSubtype, nil, nil)
	} else {
		unwidenedType = c.getCommonSupertype(baseCandidates)
	}
	return c.getWidenedType(unwidenedType)
}

func (c *Checker) getContravariantInference(inference *InferenceInfo) *Type {
	if inference.priority&InferencePriorityPriorityImpliesCombination != 0 {
		return c.getIntersectionType(inference.contraCandidates)
	}
	return c.getCommonSubtype(inference.contraCandidates)
}

func (c *Checker) unionObjectAndArrayLiteralCandidates(candidates []*Type) []*Type {
	if len(candidates) > 1 {
		objectLiterals := core.Filter(candidates, isObjectOrArrayLiteralType)
		if len(objectLiterals) != 0 {
			literalsType := c.getUnionTypeEx(objectLiterals, UnionReductionSubtype, nil, nil)
			nonLiteralTypes := core.Filter(candidates, func(t *Type) bool { return !isObjectOrArrayLiteralType(t) })
			return core.Concatenate(nonLiteralTypes, []*Type{literalsType})
		}
	}
	return candidates
}

func (c *Checker) hasPrimitiveConstraint(t *Type) bool {
	constraint := c.getConstraintOfTypeParameter(t)
	if constraint != nil {
		if constraint.flags&TypeFlagsConditional != 0 {
			constraint = c.getDefaultConstraintOfConditionalType(constraint)
		}
		return c.maybeTypeOfKind(constraint, TypeFlagsPrimitive|TypeFlagsIndex|TypeFlagsTemplateLiteral|TypeFlagsStringMapping)
	}
	return false
}

func (c *Checker) isTypeParameterAtTopLevel(t *Type, tp *Type, depth int) bool {
	return t == tp ||
		t.flags&TypeFlagsUnionOrIntersection != 0 && core.Some(t.Types(), func(t *Type) bool { return c.isTypeParameterAtTopLevel(t, tp, depth) }) ||
		depth < 3 && t.flags&TypeFlagsConditional != 0 &&
			(c.isTypeParameterAtTopLevel(c.getTrueTypeFromConditionalType(t), tp, depth+1) ||
				c.isTypeParameterAtTopLevel(c.getFalseTypeFromConditionalType(t), tp, depth+1))
}

func (c *Checker) isTypeParameterAtTopLevelInReturnType(signature *Signature, typeParameter *Type) bool {
	typePredicate := c.getTypePredicateOfSignature(signature)
	if typePredicate != nil {
		return typePredicate.t != nil && c.isTypeParameterAtTopLevel(typePredicate.t, typeParameter, 0)
	}
	return c.isTypeParameterAtTopLevel(c.getReturnTypeOfSignature(signature), typeParameter, 0)
}

func (c *Checker) getTypeFromInference(inference *InferenceInfo) *Type {
	switch {
	case inference.candidates != nil:
		return c.getUnionTypeEx(inference.candidates, UnionReductionSubtype, nil, nil)
	case inference.contraCandidates != nil:
		return c.getIntersectionType(inference.contraCandidates)
	}
	return nil
}

func getInferenceInfoForType(n *InferenceState, t *Type) *InferenceInfo {
	if t.flags&TypeFlagsTypeVariable != 0 {
		for _, inference := range n.inferences {
			if t == inference.typeParameter {
				return inference
			}
		}
	}
	return nil
}

func (c *Checker) getCommonSupertype(types []*Type) *Type {
	if len(types) == 1 {
		return types[0]
	}
	// Remove nullable types from each of the candidates.
	primaryTypes := types
	if c.strictNullChecks {
		primaryTypes = core.SameMap(types, func(t *Type) *Type {
			return c.filterType(t, func(u *Type) bool { return u.flags&TypeFlagsNullable == 0 })
		})
	}
	// When the candidate types are all literal types with the same base type, return a union
	// of those literal types. Otherwise, return the leftmost type for which no type to the
	// right is a supertype.
	var supertype *Type
	if c.literalTypesWithSameBaseType(primaryTypes) {
		supertype = c.getUnionType(primaryTypes)
	} else {
		supertype = c.getSingleCommonSupertype(primaryTypes)
	}
	// Add any nullable types that occurred in the candidates back to the result.
	if core.Same(primaryTypes, types) {
		return supertype
	}
	return c.getNullableType(supertype, c.getCombinedTypeFlags(types)&TypeFlagsNullable)
}

func (c *Checker) getSingleCommonSupertype(types []*Type) *Type {
	// First, find the leftmost type for which no type to the right is a strict supertype, and if that
	// type is a strict supertype of all other candidates, return it. Otherwise, return the leftmost type
	// for which no type to the right is a (regular) supertype.
	candidate := c.findLeftmostType(types, (*Checker).isTypeStrictSubtypeOf)
	if core.Every(types, func(t *Type) bool { return t == candidate || c.isTypeStrictSubtypeOf(t, candidate) }) {
		return candidate
	}
	return c.findLeftmostType(types, (*Checker).isTypeSubtypeOf)
}

func (c *Checker) findLeftmostType(types []*Type, f func(c *Checker, s *Type, t *Type) bool) *Type {
	var candidate *Type
	for _, t := range types {
		if candidate == nil || f(c, candidate, t) {
			candidate = t
		}
	}
	return candidate
}

// Return the leftmost type for which no type to the right is a subtype.
func (c *Checker) getCommonSubtype(types []*Type) *Type {
	var subtype *Type
	for _, t := range types {
		if subtype == nil || c.isTypeSubtypeOf(t, subtype) {
			subtype = t
		}
	}
	return subtype
}

func (c *Checker) getCombinedTypeFlags(types []*Type) TypeFlags {
	flags := TypeFlagsNone
	for _, t := range types {
		if t.flags&TypeFlagsUnion != 0 {
			flags |= c.getCombinedTypeFlags(t.Types())
		} else {
			flags |= t.flags
		}
	}
	return flags
}

func (c *Checker) literalTypesWithSameBaseType(types []*Type) bool {
	var commonBaseType *Type
	for _, t := range types {
		if t.flags&TypeFlagsNever == 0 {
			baseType := c.getBaseTypeOfLiteralType(t)
			if commonBaseType == nil {
				commonBaseType = baseType
			}
			if baseType == t || baseType != commonBaseType {
				return false
			}
		}
	}
	return true
}

func (c *Checker) isFromInferenceBlockedSource(t *Type) bool {
	return t.symbol != nil && core.Some(t.symbol.Declarations, c.isSkipDirectInferenceNode)
}

func (c *Checker) isSkipDirectInferenceNode(node *ast.Node) bool {
	return c.skipDirectInferenceNodes.Has(node)
}

// Returns `true` if `type` has the shape `[T[0]]` where `T` is `typeParameter`
func (c *Checker) isTupleOfSelf(tp *Type, t *Type) bool {
	return isTupleType(t) && c.getTupleElementType(t, 0) == c.getIndexedAccessType(tp, c.getNumberLiteralType(0)) && c.getTypeOfPropertyOfType(t, "1") == nil
}

func newInferenceInfo(typeParameter *Type) *InferenceInfo {
	return &InferenceInfo{typeParameter: typeParameter, priority: InferencePriorityMaxValue, topLevel: true, impliedArity: -1}
}

func cloneInferenceInfo(info *InferenceInfo) *InferenceInfo {
	return &InferenceInfo{
		typeParameter:    info.typeParameter,
		candidates:       slices.Clone(info.candidates),
		contraCandidates: slices.Clone(info.contraCandidates),
		inferredType:     info.inferredType,
		priority:         info.priority,
		topLevel:         info.topLevel,
		isFixed:          info.isFixed,
		impliedArity:     info.impliedArity,
	}
}

func clearCachedInferences(inferences []*InferenceInfo) {
	for _, inference := range inferences {
		if !inference.isFixed {
			inference.inferredType = nil
		}
	}
}

func hasInferenceCandidates(info *InferenceInfo) bool {
	return len(info.candidates) != 0 || len(info.contraCandidates) != 0
}

func hasInferenceCandidatesOrDefault(info *InferenceInfo) bool {
	return info.candidates != nil || info.contraCandidates != nil || hasTypeParameterDefault(info.typeParameter)
}

func hasTypeParameterDefault(tp *Type) bool {
	if tp.symbol != nil {
		for _, d := range tp.symbol.Declarations {
			if ast.IsTypeParameterDeclaration(d) && d.AsTypeParameter().DefaultType != nil {
				return true
			}
		}
	}
	return false
}

func hasOverlappingInferences(a []*InferenceInfo, b []*InferenceInfo) bool {
	for i := range a {
		if hasInferenceCandidates(a[i]) && hasInferenceCandidates(b[i]) {
			return true
		}
	}
	return false
}

func (c *Checker) mergeInferences(target []*InferenceInfo, source []*InferenceInfo) {
	for i := range target {
		if !hasInferenceCandidates(target[i]) && hasInferenceCandidates(source[i]) {
			target[i] = source[i]
		}
	}
}
