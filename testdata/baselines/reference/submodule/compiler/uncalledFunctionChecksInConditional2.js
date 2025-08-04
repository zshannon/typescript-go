//// [tests/cases/compiler/uncalledFunctionChecksInConditional2.ts] ////

//// [uncalledFunctionChecksInConditional2.ts]
{
  const perf = window.performance

  // Simplified
  if (
    perf &&
    perf.measure &&
    perf.clearMarks &&
    perf.clearMeasures
  ) {
    perf.measure("");
    perf.clearMarks("")
    perf.clearMeasures("")
  }

  // With ||
  if (
    perf &&
    perf.mark &&
    perf.measure || !!true
  ) {
    perf.mark("");
  }

  // With ??
  if (
    (
      perf &&
      perf.mark &&
      perf.measure
    ) ?? !!true
  ) {
    perf.mark("");
  }
};

// Original #49192
declare let inBrowser: boolean;
{
  let mark;
  let measure;
  const perf = inBrowser && window.performance
  /* istanbul ignore if */
  if (
    perf &&
    perf.mark &&
    perf.measure &&
    perf.clearMarks &&
    perf.clearMeasures
  ) {
    mark = (tag) => perf.mark(tag)
    measure = (name, startTag, endTag) => {
      perf.measure(name, startTag, endTag)
      perf.clearMarks(startTag)
      perf.clearMarks(endTag)
      // perf.clearMeasures(name)
    }
  }
};

let _isMobile: boolean;
function isMobile() {
  if (_isMobile === undefined) {
    const isMobileMatch =
      typeof window !== 'undefined' &&
      window.matchMedia && // no error
      window.matchMedia('(max-device-width: 680px)');
    _isMobile = isMobileMatch && isMobileMatch.matches;
  }
  return _isMobile;
}


//// [uncalledFunctionChecksInConditional2.js]
var _a;
{
    const perf = window.performance;
    // Simplified
    if (perf &&
        perf.measure &&
        perf.clearMarks &&
        perf.clearMeasures) {
        perf.measure("");
        perf.clearMarks("");
        perf.clearMeasures("");
    }
    // With ||
    if (perf &&
        perf.mark &&
        perf.measure || !!true) {
        perf.mark("");
    }
    // With ??
    if ((_a = (perf &&
        perf.mark &&
        perf.measure)) !== null && _a !== void 0 ? _a : !!true) {
        perf.mark("");
    }
}
;
{
    let mark;
    let measure;
    const perf = inBrowser && window.performance;
    /* istanbul ignore if */
    if (perf &&
        perf.mark &&
        perf.measure &&
        perf.clearMarks &&
        perf.clearMeasures) {
        mark = (tag) => perf.mark(tag);
        measure = (name, startTag, endTag) => {
            perf.measure(name, startTag, endTag);
            perf.clearMarks(startTag);
            perf.clearMarks(endTag);
            // perf.clearMeasures(name)
        };
    }
}
;
let _isMobile;
function isMobile() {
    if (_isMobile === undefined) {
        const isMobileMatch = typeof window !== 'undefined' &&
            window.matchMedia && // no error
            window.matchMedia('(max-device-width: 680px)');
        _isMobile = isMobileMatch && isMobileMatch.matches;
    }
    return _isMobile;
}
