function add(a: number, b: number): number {
    return a + b;
}

// This should cause a type error - passing string to number parameter
const result = add("hello", 5);

console.log(result);

export { add };
