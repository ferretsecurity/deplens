import * as sandcastle from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import os from "node:os";
import path from "node:path";

const hostCodexHome = process.env.CODEX_HOME
  ? path.resolve(process.env.CODEX_HOME)
  : path.join(os.homedir(), ".codex");
const hostCodexAuth = path.join(hostCodexHome, "auth.json");
const sandboxCodexMount = "/home/agent/host-codex/auth.json";
const sandboxCodexHome = "/home/agent/.codex";

const prepareCodexAuth = {
  command: [
    `mkdir -p "${sandboxCodexHome}"`,
    `test -f "${sandboxCodexMount}"`,
    `cp "${sandboxCodexMount}" "${sandboxCodexHome}/auth.json"`,
    `chmod 600 "${sandboxCodexHome}/auth.json"`,
  ].join(" && "),
};

export const codexAgent = () =>
  sandcastle.codex("gpt-5.6-terra", { effort: "medium" });

export const dockerSandbox = () =>
  docker({
    env: {
      // Sandcastle captures Codex sessions from this location. The auth setup
      // hook copies the host credential here so Codex can update it normally.
      CODEX_HOME: sandboxCodexHome,
    },
    mounts: [
      // Stage the host OAuth credential read-only. Do not mount it directly at
      // CODEX_HOME: Codex may replace auth.json atomically during token refresh.
      {
        hostPath: hostCodexAuth,
        sandboxPath: sandboxCodexMount,
        readonly: true,
      },
      // Reuse the WSL host's GitHub CLI login without copying a token into
      // .sandcastle/.env. GitHub CLI only needs to read this configuration.
      {
        hostPath: "~/.config/gh",
        sandboxPath: "/home/agent/.config/gh",
        readonly: true,
      },
    ],
  });

export const authenticationHooks = {
  sandbox: {
    onSandboxReady: [prepareCodexAuth],
  },
};

export const projectHooks = {
  sandbox: {
    onSandboxReady: [prepareCodexAuth, { command: "go mod download" }],
  },
};
