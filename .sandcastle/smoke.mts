import * as sandcastle from "@ai-hero/sandcastle";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  authenticationHooks,
  codexAgent,
  dockerSandbox,
} from "./config.mts";

const branch = `sandcastle/smoke/${Date.now()}`;
const execFileAsync = promisify(execFile);

try {
  await using sandbox = await sandcastle.createSandbox({
    branch,
    sandbox: dockerSandbox(),
    hooks: authenticationHooks,
  });

  const commands = [
    "codex login status",
    "gh auth status",
    "gh repo view --json nameWithOwner --jq .nameWithOwner",
    "go version",
  ] as const;

  const failedCommands: string[] = [];

  for (const command of commands) {
    const result = await sandbox.exec(command);
    process.stdout.write(result.stdout);
    process.stderr.write(result.stderr);

    if (result.exitCode !== 0) {
      failedCommands.push(command);
    }
  }

  if (failedCommands.length > 0) {
    throw new Error(
      `Sandbox prerequisite checks failed: ${failedCommands.join(", ")}`,
    );
  }

  let result: Awaited<ReturnType<typeof sandbox.run>>;
  try {
    result = await sandbox.run({
      name: "authentication-smoke",
      agent: codexAgent(),
      maxIterations: 1,
      promptFile: "./.sandcastle/smoke-prompt.md",
      logging: { type: "stdout" },
    });
  } catch (error) {
    throw new Error(
      "Codex could not complete the authentication smoke prompt. Re-run `codex login` on the WSL host, then try again.",
      { cause: error },
    );
  }

  process.stdout.write(result.stdout);

  if (!result.stdout.includes("<smoke>PASS</smoke>")) {
    throw new Error("Sandcastle authentication smoke test failed.");
  }

  console.log("Sandcastle authentication smoke test passed.");
} finally {
  // createSandbox intentionally keeps named branches. Remove this branch only
  // when it is fully merged; `-d` refuses cleanup if the smoke run committed.
  try {
    await execFileAsync("git", ["branch", "-d", branch], {
      cwd: process.cwd(),
    });
  } catch {
    console.warn(`Smoke branch retained for inspection: ${branch}`);
  }
}
