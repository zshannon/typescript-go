import assert from "node:assert/strict";
import test from "node:test";
import {
    type ContentMapperContribution,
    validateContentMapperRegistration,
} from "../src/contentMapperContributions";

test("rejects extensions that the server cannot register as content mappers", () => {
    for (const extension of [".ts", ".json", ".foo/bar"]) {
        assert.throws(
            () => validateContentMapperRegistration("test", [contribution(extension)]),
            new TypeError(`Content mapper contribution has invalid extension '${extension}'.`),
        );
    }
});

test("rejects extension collisions within and across registrations", () => {
    assert.throws(
        () => validateContentMapperRegistration("test", [contribution(".vue"), contribution(".vue")]),
        new TypeError("Content mapper contributions both claim extension '.vue'."),
    );

    const registrations = new Map([
        ["existing", [contribution(".vue")]],
    ]);
    assert.throws(
        () => validateContentMapperRegistration("test", [contribution(".vue")], registrations),
        new TypeError("Content mapper contributions both claim extension '.vue'."),
    );
});

test("accepts custom extensions without interpreting mapper executable arguments", () => {
    assert.doesNotThrow(() => validateContentMapperRegistration("test", [{
        extensions: [".vue"],
        inferredProject: {
            manifest: {
                name: "vue-mapper",
                exec: ["esbuild", "--future-flag", "tsc", "--another-future-flag"],
            },
        },
    }]));
});

function contribution(extension: string): ContentMapperContribution {
    return {
        extensions: [extension],
        inferredProject: {
            manifest: {
                name: "test-mapper",
                exec: ["mapper"],
            },
        },
    };
}
