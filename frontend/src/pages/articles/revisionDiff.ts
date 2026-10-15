export type DiffLineType = 'same' | 'add' | 'remove';

export interface DiffLine {
  type: DiffLineType;
  text: string;
}

// 行级 diff：基于最长公共子序列（LCS），不依赖第三方库。
// 正文通常几十到几百行，O(n*m) 的 DP 空间开销可接受。
export function diffLines(oldText: string, newText: string): DiffLine[] {
  const oldLines = oldText.split('\n');
  const newLines = newText.split('\n');
  const oldCount = oldLines.length;
  const newCount = newLines.length;

  const table: number[][] = Array.from({ length: oldCount + 1 }, () => new Array<number>(newCount + 1).fill(0));
  for (let i = oldCount - 1; i >= 0; i -= 1) {
    for (let j = newCount - 1; j >= 0; j -= 1) {
      if (oldLines[i] === newLines[j]) {
        table[i][j] = table[i + 1][j + 1] + 1;
      } else {
        table[i][j] = Math.max(table[i + 1][j], table[i][j + 1]);
      }
    }
  }

  const result: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < oldCount && j < newCount) {
    if (oldLines[i] === newLines[j]) {
      result.push({ type: 'same', text: oldLines[i] });
      i += 1;
      j += 1;
    } else if (table[i + 1][j] >= table[i][j + 1]) {
      result.push({ type: 'remove', text: oldLines[i] });
      i += 1;
    } else {
      result.push({ type: 'add', text: newLines[j] });
      j += 1;
    }
  }
  while (i < oldCount) {
    result.push({ type: 'remove', text: oldLines[i] });
    i += 1;
  }
  while (j < newCount) {
    result.push({ type: 'add', text: newLines[j] });
    j += 1;
  }
  return result;
}
