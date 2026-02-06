import * as core from '@actions/core';
import * as github from '@actions/github';
import { analyzeMigrations } from './analyzer';
import { commentOnPR } from './pr-comment';

async function run(): Promise<void> {
  try {
    // Get inputs
    const migrationPath = core.getInput('migration-path', { required: true });
    const dbUrl = core.getInput('db-url');
    const dbType = core.getInput('db-type') || 'postgresql';
    const failOnRisk = core.getInput('fail-on-risk');
    const githubToken = core.getInput('github-token', { required: true });
    const comprehensive = core.getInput('comprehensive') === 'true';

    core.info(`Analyzing migrations at: ${migrationPath}`);

    // Run DMA analysis
    const result = await analyzeMigrations({
      migrationPath,
      dbUrl,
      dbType,
      comprehensive,
      failOnRisk,
    });

    // Set outputs
    core.setOutput('risk-level', result.maxRiskLevel);
    core.setOutput('total-operations', result.totalOperations);

    // Comment on PR if in pull request context
    const pr = github.context.payload.pull_request;
    if (pr) {
      await commentOnPR(githubToken, result);
    }

    // Fail if risk threshold exceeded
    if (failOnRisk && result.shouldFail) {
      core.setFailed(
        `Migration risk level '${result.maxRiskLevel}' exceeds threshold '${failOnRisk}'`
      );
    }

    core.info('Analysis complete!');
  } catch (error) {
    if (error instanceof Error) {
      core.setFailed(error.message);
    }
  }
}

run();
