import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const ROOT = path.resolve(process.cwd(), 'src', 'components', 'admin');
const TARGET_FILES = [
  'admin-shell.tsx',
  'dashboard-page.tsx',
  'knowledge-bases-page.tsx',
  'knowledge-base-detail-page.tsx',
  'create-knowledge-base-modal.tsx',
  'retrieval-lab-page.tsx',
  'retrieval-debug-page.tsx',
  'retrieval-logs-page.tsx',
  'ingest-logs-page.tsx',
  'quality-monitor-page.tsx',
  'evaluation-datasets-page.tsx',
  'evaluation-runs-page.tsx',
  'evaluation-report-page.tsx',
  'strategy-center-page.tsx',
  'cost-ops-cost-page.tsx',
  'audit-page.tsx',
  'alerts-page.tsx',
  'weekly-reports-page.tsx',
];

const FORBIDDEN_PATTERNS = [
  /Phase 3/i,
  /Phase 4/i,
  /\bP3\b/,
  /\bP4\b/,
  /Feature Flags/,
  /Contract gap/,
];

function readTargetFiles() {
  return TARGET_FILES.map((fileName) => {
    const absolutePath = path.join(ROOT, fileName);
    return {
      fileName,
      content: fs.readFileSync(absolutePath, 'utf8'),
    };
  });
}

describe('admin UI copy guard', () => {
  it('does not expose internal stage vocabulary in key admin pages', () => {
    const violations = readTargetFiles().flatMap(({ fileName, content }) =>
      FORBIDDEN_PATTERNS.filter((pattern) => pattern.test(content)).map((pattern) => ({
        fileName,
        pattern: pattern.toString(),
      }))
    );

    expect(violations).toEqual([]);
  });
});
