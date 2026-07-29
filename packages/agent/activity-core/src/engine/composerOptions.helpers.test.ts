import assert from "node:assert/strict";
import test from "node:test";
import { composerOptionsRequestSignature } from "./composerOptions.helpers.ts";

test("composer options request signature preserves opaque unknown model parameters", () => {
  const left = composerOptionsRequestSignature({
    provider: " cursor ",
    cwd: " /workspace ",
    settings: {
      model: " composer-2.5 ",
      modelParameters: { future: " opaque ", context: " 1m " }
    }
  });
  const reordered = composerOptionsRequestSignature({
    provider: "cursor",
    cwd: "/workspace",
    settings: {
      model: "composer-2.5",
      modelParameters: { context: " 1m ", future: " opaque " }
    }
  });
  const changed = composerOptionsRequestSignature({
    provider: "cursor",
    cwd: "/workspace",
    settings: {
      model: "composer-2.5",
      modelParameters: { context: "200k", future: " opaque " }
    }
  });

  assert.equal(left, reordered);
  assert.notEqual(left, changed);
  assert.equal(
    JSON.parse(left).settings.modelParameters.future,
    " opaque "
  );
});
