// Feature Pipeline — select, implement, review, integrate, and close one issue
// at a time on the currently checked-out feature branch.

import * as sandcastle from "@ai-hero/sandcastle";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { codexAgent, dockerSandbox, projectHooks } from "./config.mts";

const execFileAsync = promisify(execFile);

// Bound unattended runs while still allowing newly unblocked tickets to be
// picked up after their blocker is integrated and closed.
const MAX_ITERATIONS = 10;
const SANDCASTLE_LABEL = "Sandcastle";

type Issue = {
  number: number;
  title: string;
  body: string;
  labels: Array<{ name: string }>;
  comments: Array<{ body: string }>;
};

type Repository = {
  nameWithOwner: string;
  defaultBranchRef: { name: string };
};

type CommandFailure = Error & {
  code?: number | string;
  stdout?: string;
  stderr?: string;
};

const run = async (
  command: string,
  args: string[],
  printOutput = false,
) => {
  const result = await execFileAsync(command, args, {
    cwd: process.cwd(),
    encoding: "utf8",
    maxBuffer: 10 * 1024 * 1024,
  });

  if (printOutput && result.stdout) {
    process.stdout.write(result.stdout);
  }
  if (printOutput && result.stderr) {
    process.stderr.write(result.stderr);
  }

  return result.stdout;
};

const commandFailureDetails = (error: unknown): string => {
  if (!(error instanceof Error)) {
    return String(error);
  }

  const failure = error as CommandFailure;
  const output = [failure.stdout, failure.stderr]
    .filter((value): value is string => Boolean(value?.trim()))
    .join("\n")
    .trim();

  return output ? `${failure.message}\n${output}` : failure.message;
};

const git = (args: string[]) => run("git", args);
const gh = (args: string[]) => run("gh", args);

const getRepository = async (): Promise<Repository> => {
  const output = await gh([
    "repo",
    "view",
    "--json",
    "nameWithOwner,defaultBranchRef",
  ]);
  return JSON.parse(output) as Repository;
};

const getFeatureBranch = async (repository: Repository): Promise<string> => {
  let branch: string;
  try {
    branch = (await git(["symbolic-ref", "--quiet", "--short", "HEAD"])).trim();
  } catch {
    throw new Error(
      "sandcastle:feature requires a checked-out feature branch; HEAD is detached.",
    );
  }

  if (branch === repository.defaultBranchRef.name) {
    throw new Error(
      `Refusing to integrate Sandcastle tickets directly into the default branch (${branch}).`,
    );
  }

  return branch;
};

const listSandcastleIssues = async (): Promise<Issue[]> => {
  const output = await gh([
    "issue",
    "list",
    "--state",
    "open",
    "--label",
    SANDCASTLE_LABEL,
    "--limit",
    "100",
    "--json",
    "number,title,body,labels,comments",
  ]);
  const issues = JSON.parse(output) as Issue[];
  return issues.sort((left, right) => left.number - right.number);
};

const fallbackBlockerNumbers = (body: string): number[] => {
  const blockers = new Set<number>();
  const lines = body.split(/\r?\n/);
  let inBlockedBySection = false;

  for (const line of lines) {
    if (/^##\s+Blocked by\s*$/i.test(line.trim())) {
      inBlockedBySection = true;
      continue;
    }

    if (inBlockedBySection && /^##\s+/.test(line.trim())) {
      inBlockedBySection = false;
    }

    const inline = line.match(/^\s*Blocked by:\s*(.*)$/i);
    if (!inBlockedBySection && !inline) {
      continue;
    }

    const text = inline?.[1] ?? line;
    for (const match of text.matchAll(/#(\d+)/g)) {
      blockers.add(Number(match[1]));
    }
  }

  return [...blockers];
};

const issueIsOpen = async (
  issueNumber: number,
  stateByIssue: Map<number, boolean>,
): Promise<boolean> => {
  const cached = stateByIssue.get(issueNumber);
  if (cached !== undefined) {
    return cached;
  }

  const output = await gh([
    "issue",
    "view",
    String(issueNumber),
    "--json",
    "state",
  ]);
  const { state } = JSON.parse(output) as { state: string };
  const open = state.toUpperCase() === "OPEN";
  stateByIssue.set(issueNumber, open);
  return open;
};

const issueIsBlocked = async (
  issue: Issue,
  repository: Repository,
  stateByIssue: Map<number, boolean>,
): Promise<boolean> => {
  const output = await gh([
    "api",
    `repos/${repository.nameWithOwner}/issues/${issue.number}`,
  ]);
  const details = JSON.parse(output) as {
    issue_dependencies_summary?: { blocked_by?: number };
  };
  const nativeBlockedBy = details.issue_dependencies_summary?.blocked_by;

  if (typeof nativeBlockedBy === "number") {
    return nativeBlockedBy > 0;
  }

  const blockerNumbers = fallbackBlockerNumbers(issue.body);
  for (const blockerNumber of blockerNumbers) {
    if (await issueIsOpen(blockerNumber, stateByIssue)) {
      return true;
    }
  }

  return false;
};

const selectIssue = async (
  repository: Repository,
): Promise<{ issue?: Issue; blockedIssueNumbers: number[] }> => {
  const issues = await listSandcastleIssues();
  const blockedIssueNumbers: number[] = [];
  const stateByIssue = new Map<number, boolean>();

  for (const issue of issues) {
    if (!(await issueIsOpen(issue.number, stateByIssue))) {
      continue;
    }

    if (await issueIsBlocked(issue, repository, stateByIssue)) {
      blockedIssueNumbers.push(issue.number);
      continue;
    }

    return { issue, blockedIssueNumbers };
  }

  return { blockedIssueNumbers };
};

const commentOnIssue = (issueNumber: number, body: string) =>
  gh(["issue", "comment", String(issueNumber), "--body", body]);

const mergeInProgress = async (): Promise<boolean> => {
  try {
    await git(["rev-parse", "--quiet", "--verify", "MERGE_HEAD"]);
    return true;
  } catch {
    return false;
  }
};

const mergeTicket = async (
  issue: Issue,
  ticketBranch: string,
  featureBranch: string,
) => {
  try {
    await git([
      "merge",
      "--no-ff",
      "-m",
      `Merge Sandcastle issue #${issue.number}: ${issue.title}`,
      ticketBranch,
    ]);
  } catch (error) {
    if (!(await mergeInProgress())) {
      throw error;
    }

    await git(["merge", "--abort"]);
    await commentOnIssue(
      issue.number,
      [
        `Sandcastle could not merge \`${ticketBranch}\` into \`${featureBranch}\` because of conflicts.`,
        "The merge was aborted and the issue remains open for manual recovery.",
      ].join("\n\n"),
    );
    throw new Error(
      `Merge conflict integrating issue #${issue.number}. The merge was aborted.\n${commandFailureDetails(error)}`,
    );
  }
};

const verifyFeatureBranch = async (
  issue: Issue,
  ticketBranch: string,
  featureBranch: string,
) => {
  const checks: Array<[string, string[]]> = [
    ["go", ["test", "./..."]],
    ["go", ["vet", "./..."]],
  ];

  for (const [command, args] of checks) {
    const displayCommand = [command, ...args].join(" ");
    try {
      await run(command, args, true);
    } catch (error) {
      await commentOnIssue(
        issue.number,
        [
          `Sandcastle merged \`${ticketBranch}\` into \`${featureBranch}\`, but \`${displayCommand}\` failed.`,
          "The merge commit remains on the feature branch and the issue remains open for manual recovery.",
        ].join("\n\n"),
      );
      throw new Error(
        `Integration verification failed for issue #${issue.number}: ${displayCommand}\n${commandFailureDetails(error)}`,
      );
    }
  }
};

const closeIssue = (issueNumber: number) =>
  gh([
    "issue",
    "close",
    String(issueNumber),
    "--comment",
    "Completed by Sandcastle",
  ]);

const repository = await getRepository();
const featureBranch = await getFeatureBranch(repository);

console.log(`\nIntegrating Sandcastle tickets into feature branch: ${featureBranch}`);

for (let iteration = 1; iteration <= MAX_ITERATIONS; iteration++) {
  console.log(`\n=== Iteration ${iteration}/${MAX_ITERATIONS} ===\n`);

  const selection = await selectIssue(repository);
  if (!selection.issue) {
    if (selection.blockedIssueNumbers.length) {
      console.log(
        `No eligible Sandcastle issues. Blocked issues: ${selection.blockedIssueNumbers
          .map((number) => `#${number}`)
          .join(", ")}`,
      );
    } else {
      console.log("No open Sandcastle issues.");
    }
    break;
  }

  const issue = selection.issue;
  const ticketBranch = `sandcastle/sequential-reviewer/${Date.now()}`;
  console.log(`Selected issue #${issue.number}: ${issue.title}`);

  const sandbox = await sandcastle.createSandbox({
    branch: ticketBranch,
    baseBranch: featureBranch,
    sandbox: dockerSandbox(),
    hooks: projectHooks,
  });

  let implementationComplete = false;
  try {
    const implement = await sandbox.run({
      name: "implementer",
      maxIterations: 1,
      agent: codexAgent(),
      promptFile: "./.sandcastle/feature-implement-prompt.md",
      promptArgs: {
        ISSUE: JSON.stringify(issue, null, 2),
      },
    });

    if (!implement.commits.length) {
      console.log(
        `Implementation agent made no commits for issue #${issue.number}. Stopping.`,
      );
    } else {
      implementationComplete = true;
      console.log(`\nImplementation complete on branch: ${ticketBranch}`);
      console.log(`Commits: ${implement.commits.length}`);

      await sandbox.run({
        name: "reviewer",
        maxIterations: 1,
        agent: codexAgent(),
        promptFile: "./.sandcastle/review-prompt.md",
        promptArgs: {
          BRANCH: ticketBranch,
        },
      });

      console.log("\nReview complete.");
    }
  } finally {
    await sandbox.close();
  }

  if (!implementationComplete) {
    break;
  }

  await mergeTicket(issue, ticketBranch, featureBranch);
  await verifyFeatureBranch(issue, ticketBranch, featureBranch);
  await closeIssue(issue.number);
  await git(["branch", "-d", ticketBranch]);

  console.log(`\nIntegrated and closed issue #${issue.number}.`);
}

console.log("\nAll done.");
