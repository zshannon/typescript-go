currentDirectory::/home/src/workspaces/project
useCaseSensitiveFileNames::true
Input::
//// [/home/src/workspaces/project/a.ts] *new* 
const a = class { private p = 10; };
//// [/home/src/workspaces/project/tsconfig.json] *new* 
{
	"compilerOptions": {
            "noEmit": true
	}
}

tsgo -w
ExitStatus:: Success
Output::
//// [/home/src/tslibs/TS/Lib/lib.d.ts] *Lib*
/// <reference no-default-lib="true"/>
interface Boolean {}
interface Function {}
interface CallableFunction {}
interface NewableFunction {}
interface IArguments {}
interface Number { toExponential: any; }
interface Object {}
interface RegExp {}
interface String { charAt: any; }
interface Array<T> { length: number; [n: number]: T; }
interface ReadonlyArray<T> {}
interface SymbolConstructor {
    (desc?: string | number): symbol;
    for(name: string): symbol;
    readonly toStringTag: symbol;
}
declare var Symbol: SymbolConstructor;
interface Symbol {
    readonly [Symbol.toStringTag]: string;
}
declare const console: { log(msg: any): void; };

SemanticDiagnostics::
*refresh*    /home/src/tslibs/TS/Lib/lib.d.ts
*refresh*    /home/src/workspaces/project/a.ts
Signatures::


Edit [0]:: fix error
//// [/home/src/workspaces/project/a.ts] *modified* 
const a = "hello";


Output::

SemanticDiagnostics::
*refresh*    /home/src/tslibs/TS/Lib/lib.d.ts
*refresh*    /home/src/workspaces/project/a.ts
Signatures::
(computed .d.ts) /home/src/workspaces/project/a.ts


Edit [1]:: emit after fixing error
//// [/home/src/workspaces/project/tsconfig.json] *modified* 
{
	"compilerOptions": {
            
	}
}


Output::
//// [/home/src/workspaces/project/a.js] *new* 
const a = "hello";


SemanticDiagnostics::
Signatures::


Edit [2]:: no emit run after fixing error
//// [/home/src/workspaces/project/tsconfig.json] *modified* 
{
	"compilerOptions": {
            "noEmit": true,
            
	}
}


Output::

SemanticDiagnostics::
Signatures::


Edit [3]:: introduce error
//// [/home/src/workspaces/project/a.ts] *modified* 
const a = class { private p = 10; };


Output::

SemanticDiagnostics::
*refresh*    /home/src/tslibs/TS/Lib/lib.d.ts
*refresh*    /home/src/workspaces/project/a.ts
Signatures::
(computed .d.ts) /home/src/workspaces/project/a.ts


Edit [4]:: emit when error
//// [/home/src/workspaces/project/tsconfig.json] *modified* 
{
	"compilerOptions": {
            
	}
}


Output::
//// [/home/src/workspaces/project/a.js] *modified* 
const a = class {
    p = 10;
};


SemanticDiagnostics::
Signatures::


Edit [5]:: no emit run when error
//// [/home/src/workspaces/project/tsconfig.json] *modified* 
{
	"compilerOptions": {
            "noEmit": true,
            
	}
}


Output::

SemanticDiagnostics::
Signatures::
