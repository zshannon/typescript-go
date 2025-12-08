// Minimal React stub for type checking tests
module.exports = {
  useState: () => [null, () => {}],
  useEffect: () => {},
  useCallback: (fn) => fn,
  useMemo: (fn) => fn(),
  useRef: (v) => ({ current: v }),
  createContext: (v) => ({ Provider: () => null, Consumer: () => null }),
};
