export type PreviewBlock =
  | { type: 'heading'; level: 1 | 2 | 3; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'quote'; text: string }
  | { type: 'list'; items: string[] }
  | { type: 'table'; headers: string[]; rows: string[][] }
  | { type: 'code'; code: string };

export function parsePreviewBlocks(source: string): PreviewBlock[] {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks: PreviewBlock[] = [];
  let paragraph: string[] = [];
  let listItems: string[] = [];
  let codeLines: string[] = [];
  let tableLines: string[] = [];
  let inCode = false;

  const flushParagraph = () => {
    if (paragraph.length === 0) return;
    blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
    paragraph = [];
  };

  const flushList = () => {
    if (listItems.length === 0) return;
    blocks.push({ type: 'list', items: listItems });
    listItems = [];
  };

  const flushTable = () => {
    const table = parseTable(tableLines);
    if (table) blocks.push(table);
    tableLines = [];
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      if (inCode) {
        blocks.push({ type: 'code', code: codeLines.join('\n') });
        codeLines = [];
        inCode = false;
      } else {
        flushParagraph();
        flushList();
        flushTable();
        inCode = true;
      }
      continue;
    }

    if (inCode) {
      codeLines.push(line);
      continue;
    }

    if (!trimmed) {
      flushParagraph();
      flushList();
      flushTable();
      continue;
    }

    if (isPotentialTableLine(trimmed)) {
      flushParagraph();
      flushList();
      tableLines.push(trimmed);
      continue;
    }

    flushTable();

    const heading = /^(#{1,3})\s+(.+)$/.exec(trimmed);
    if (heading) {
      flushParagraph();
      flushList();
      blocks.push({
        type: 'heading',
        level: heading[1].length as 1 | 2 | 3,
        text: heading[2],
      });
      continue;
    }

    const listItem = /^[-*]\s+(.+)$/.exec(trimmed);
    if (listItem) {
      flushParagraph();
      listItems.push(listItem[1]);
      continue;
    }

    if (trimmed.startsWith('>')) {
      flushParagraph();
      flushList();
      blocks.push({ type: 'quote', text: trimmed.replace(/^>\s?/, '') });
      continue;
    }

    paragraph.push(trimmed);
  }

  flushParagraph();
  flushList();
  flushTable();
  if (codeLines.length > 0) {
    blocks.push({ type: 'code', code: codeLines.join('\n') });
  }

  return blocks;
}

function isPotentialTableLine(line: string) {
  return line.includes('|');
}

function parseTable(lines: string[]): PreviewBlock | null {
  if (lines.length < 2 || !isTableDivider(lines[1])) {
    return lines.length > 0 ? { type: 'paragraph', text: lines.join(' ') } : null;
  }

  const headers = splitTableCells(lines[0]);
  const columnCount = headers.length;
  if (columnCount === 0) return { type: 'paragraph', text: lines.join(' ') };

  const rows = lines.slice(2).map((line) => {
    const cells = splitTableCells(line);
    return Array.from({ length: columnCount }, (_, index) => cells[index] ?? '');
  });

  return { type: 'table', headers, rows };
}

function isTableDivider(line: string) {
  const cells = splitTableCells(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell.trim()));
}

function splitTableCells(line: string) {
  const normalized = line.trim().replace(/^\|/, '').replace(/\|$/, '');
  return normalized.split('|').map((cell) => cell.trim());
}
