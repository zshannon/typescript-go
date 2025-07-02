# Claude Development Notes

This file contains development patterns and approaches discovered during Claude-assisted development.

## C Bridge Enum Implementation Pattern

### User's Clean Approach (Preferred)

When implementing C bridge enums for Go types (like esbuild Platform), follow this clean pattern:

#### **Core Philosophy**: 
- **Simplicity over complexity** - avoid unnecessary abstractions
- **Direct module integration** - use proper imports instead of manual declarations
- **Modern Swift patterns** - leverage language features elegantly

#### **File Organization**
```
✅ ESBuildTypes.swift (extensible for future types)
❌ ESBuildPlatform.swift (too specific)
```

#### **Swift Enum Structure**
```swift
import Foundation
import TSCBridge  // ← Direct module import, no manual declarations

public enum ESBuildPlatform: Int32, CaseIterable {
    case `default`    // ← No placeholder raw values
    case browser      // ← Let Swift assign automatic values  
    case node
    case neutral
    
    // Simple computed property using C bridge directly
    public var actualRawValue: Int32 {
        switch self {
        case .default: return esbuild_platform_default()
        case .browser: return esbuild_platform_browser()
        case .node: return esbuild_platform_node()
        case .neutral: return esbuild_platform_neutral()
        }
    }
}
```

#### **Avoid Over-Engineering**
```swift
❌ Complex approach:
case `default` = -1  // Placeholder values
case browser = -2    // Unnecessary complexity

✅ Clean approach:
case `default`       // Let Swift handle raw values
case browser         // Use C bridge for actual values
```

#### **Test Pattern - Elegant Validation**
```swift
@Test("All C bridge values implemented")
func testAllCPlatformValuesImplemented() {
    let cArrayPtr = esbuild_get_all_platform_values()
    defer { esbuild_free_int_array(cArrayPtr) }
    
    guard let cArrayPtr else { return }  // ← Modern guard syntax
    
    // Simple, direct validation
    #expect(cPlatformValues.count == ESBuildPlatform.allCases.count)
    for value in cPlatformValues.sorted() {
        #expect(value == ESBuildPlatform(rawValue: value)?.rawValue)
    }
}
```

#### **Key Principles to Follow**:

1. **🚀 Start Simple**: Begin with direct C bridge integration, no abstractions
2. **📦 Use Module Imports**: `import TSCBridge` instead of `@_silgen_name` declarations  
3. **🎯 File Naming**: Use extensible names (`ESBuildTypes` not `ESBuildPlatform`)
4. **✨ Modern Swift**: Use current syntax (`guard let variable` not `guard let newVariable = variable`)
5. **🔄 Direct Validation**: Simple loops instead of complex set operations
6. **⚡ No Premature Optimization**: Avoid placeholder values and complex abstractions

#### **Future C Bridge Enum Checklist**:
- [ ] Name file for extensibility (`*Types.swift`)
- [ ] Import `TSCBridge` module directly
- [ ] Use automatic Swift raw values
- [ ] Create simple `actualRawValue` computed property
- [ ] Write concise test validation
- [ ] Use modern Swift syntax throughout

This pattern scales beautifully for additional esbuild types (Format, Target, etc.) while maintaining simplicity and readability.

## Build Commands

To rebuild the C bridge after Go changes:
```bash
make
```

To run Swift tests:
```bash
swift test --filter ESBuildTypesTests
```