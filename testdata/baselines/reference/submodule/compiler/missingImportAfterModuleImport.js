//// [tests/cases/compiler/missingImportAfterModuleImport.ts] ////

//// [missingImportAfterModuleImport_0.ts]
declare module "SubModule" {
    class SubModule {
        public static StaticVar: number;
        public InstanceVar: number;
        constructor();
    }
    export = SubModule;
}

//// [missingImportAfterModuleImport_1.ts]
///<reference path='missingImportAfterModuleImport_0.ts' preserve="true" />
import SubModule = require('SubModule');
class MainModule {
    // public static SubModule: SubModule;
    public SubModule: SubModule;
    constructor() { }
}
export = MainModule;



//// [missingImportAfterModuleImport_0.js]
//// [missingImportAfterModuleImport_1.js]
"use strict";
class MainModule {
    // public static SubModule: SubModule;
    SubModule;
    constructor() { }
}
module.exports = MainModule;


//// [missingImportAfterModuleImport_0.d.ts]
declare module "SubModule" {
    class SubModule {
        static StaticVar: number;
        InstanceVar: number;
        constructor();
    }
    export = SubModule;
}
//// [missingImportAfterModuleImport_1.d.ts]
/// <reference path="missingImportAfterModuleImport_0.d.ts" preserve="true" />
import SubModule = require('SubModule');
declare class MainModule {
    // public static SubModule: SubModule;
    SubModule: SubModule;
    constructor();
}
export = MainModule;
