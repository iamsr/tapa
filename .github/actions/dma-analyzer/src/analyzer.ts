import * as exec from '@actions/exec';
import * as core from '@actions/core';
import * as fs from 'fs';

export interface AnalysisOptions {
  migrationPath: string;
  dbUrl?: string;
  dbType: string;
  comprehensive: boolean;
  failOnRisk?: string;
}

export interface AnalysisResult {
  maxRiskLevel: string;
  totalOperations: number;
  shouldFail: boolean;
  jsonOutput: any;
  summary: string;
}

export async function analyzeMigrations(
  options: AnalysisOptions
): Promise<AnalysisResult> {
  // Install DMA if not present
  await installDMA();

  // Build command
  const args = [
    'analyze',
    options.migrationPath,
    '--format', 'json',
  ];

  if (options.dbUrl) {
    args.push('--db', options.dbUrl);
  } else {
    args.push('--dry-run');
  }

  if (options.dbType) {
    args.push('--db-type', options.dbType);
  }

  if (options.comprehensive) {
    args.push('--comprehensive');
  }

  if (options.failOnRisk) {
    args.push('--fail-on-risk-level', options.failOnRisk);
  }

  // Execute DMA
  let output = '';
  let error = '';

  const exitCode = await exec.exec('dma', args, {
    listeners: {
      stdout: (data: Buffer) => {
        output += data.toString();
      },
      stderr: (data: Buffer) => {
        error += data.toString();
      },
    },
    ignoreReturnCode: true,
  });

  if (exitCode !== 0 && !output) {
    throw new Error(`DMA failed: ${error}`);
  }

  // Parse JSON output
  const jsonOutput = JSON.parse(output);

  // Calculate max risk
  let maxRiskLevel = 'low';
  let totalOps = 0;

  for (const migration of jsonOutput.Migrations) {
    totalOps += migration.Operations.length;
    for (const op of migration.Operations) {
      const risk = getRiskLevel(op.RiskScore);
      if (compareRisk(risk, maxRiskLevel) > 0) {
        maxRiskLevel = risk;
      }
    }
  }

  // Generate summary
  const summary = generateSummary(jsonOutput);

  const shouldFail = options.failOnRisk
    ? compareRisk(maxRiskLevel, options.failOnRisk) > 0
    : false;

  return {
    maxRiskLevel,
    totalOperations: totalOps,
    shouldFail,
    jsonOutput,
    summary,
  };
}

async function installDMA(): Promise<void> {
  core.info('Installing DMA...');
  await exec.exec('go', ['install', 'github.com/yourusername/dma/cmd/dma@latest']);
}

function getRiskLevel(score: number): string {
  if (score >= 76) return 'critical';
  if (score >= 51) return 'high';
  if (score >= 26) return 'medium';
  return 'low';
}

function compareRisk(a: string, b: string): number {
  const levels: Record<string, number> = { low: 0, medium: 1, high: 2, critical: 3 };
  return levels[a] - levels[b];
}

function generateSummary(jsonOutput: any): string {
  let summary = '## 🔍 Migration Analysis Results\n\n';

  let totalOps = 0;
  let highRiskOps = 0;

  for (const migration of jsonOutput.Migrations) {
    totalOps += migration.Operations.length;
    for (const op of migration.Operations) {
      if (op.RiskScore >= 51) {
        highRiskOps++;
      }
    }
  }

  summary += `**Total Operations:** ${totalOps}\n`;
  summary += `**High Risk Operations:** ${highRiskOps}\n\n`;

  // Add table of operations
  summary += '### Operations\n\n';
  summary += '| Operation | Table | Risk | Lock Type | Estimated Time |\n';
  summary += '|-----------|-------|------|-----------|----------------|\n';

  for (const migration of jsonOutput.Migrations) {
    for (const op of migration.Operations) {
      const riskEmoji = op.RiskScore >= 76 ? '🔴' : op.RiskScore >= 51 ? '🟠' : op.RiskScore >= 26 ? '🟡' : '🟢';
      summary += `| ${riskEmoji} ${op.Type} | ${op.TableName || 'N/A'} | ${op.RiskScore} | ${op.LockType} | ${op.EstimatedTimeSeconds}s |\n`;
    }
  }

  return summary;
}
