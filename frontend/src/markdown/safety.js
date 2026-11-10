const ALLOWED_COMPONENTS = new Set(['Figure', 'Gallery', 'br', 'u']);
const EXECUTABLE_NODE_TYPES = new Set(['mdxjsEsm', 'mdxFlowExpression', 'mdxTextExpression']);

export function remarkMdxSafety() {
  return (tree) => {
    visitNode(tree);
  };
}

function visitNode(node) {
  if (!node || typeof node !== 'object') {
    return;
  }

  if (
    (node.type === 'link' || node.type === 'image' || node.type === 'definition') &&
    !isSafeContentUrl(node.url, node.type === 'image')
  ) {
    throw new Error('不支持的链接协议');
  }

  if (EXECUTABLE_NODE_TYPES.has(node.type)) {
    rejectNode(node.type, node);
  }

  if (node.type === 'mdxJsxFlowElement' || node.type === 'mdxJsxTextElement') {
    validateElement(node);
  }

  if (Array.isArray(node.children)) {
    for (const child of node.children) {
      visitNode(child);
    }
  }
}

function validateElement(node) {
  if (!ALLOWED_COMPONENTS.has(node.name)) {
    rejectNode(node.type, node);
  }

  if (['br', 'u'].includes(node.name) && node.attributes?.length) {
    rejectNode('mdxJsxAttribute', node);
  }

  for (const attribute of node.attributes ?? []) {
    if (attribute.type !== 'mdxJsxAttribute') {
      rejectNode(attribute.type || 'mdxJsxAttribute');
    }

    if (typeof attribute.name !== 'string') {
      rejectNode(attribute.type);
    }

    if (typeof attribute.value === 'string') {
      continue;
    }

    if (
      node.name === 'Gallery' &&
      attribute.name === 'items' &&
      isSafeLiteralAttribute(attribute.value)
    ) {
      continue;
    }

    rejectNode(attribute.type);
  }
}

function isSafeLiteralAttribute(value) {
  const program = value?.data?.estree;
  return (
    value?.type === 'mdxJsxAttributeValueExpression' &&
    program?.type === 'Program' &&
    program.body?.length === 1 &&
    program.body[0]?.type === 'ExpressionStatement' &&
    isSafeLiteralExpression(program.body[0].expression)
  );
}

function isSafeLiteralExpression(node) {
  if (!node || typeof node !== 'object') {
    return false;
  }

  if (node.type === 'Literal') {
    return (
      typeof node.value === 'string' ||
      typeof node.value === 'number' ||
      typeof node.value === 'boolean' ||
      node.value === null
    );
  }

  if (node.type === 'ArrayExpression') {
    return node.elements.every((element) => element !== null && isSafeLiteralExpression(element));
  }

  if (node.type === 'ObjectExpression') {
    return node.properties.every(isSafeLiteralProperty);
  }

  if (
    node.type === 'UnaryExpression' &&
    node.operator === '-' &&
    typeof node.argument?.value === 'number'
  ) {
    return isSafeLiteralExpression(node.argument);
  }

  return false;
}

function isSafeLiteralProperty(node) {
  if (
    node?.type !== 'Property' ||
    node.kind !== 'init' ||
    node.computed ||
    node.method ||
    node.shorthand
  ) {
    return false;
  }

  const key = propertyKey(node.key);
  return (
    key !== null &&
    !['__proto__', 'constructor', 'prototype'].includes(key) &&
    isSafeLiteralExpression(node.value)
  );
}

function propertyKey(node) {
  if (node?.type === 'Identifier') {
    return node.name;
  }

  if (
    node?.type === 'Literal' &&
    (typeof node.value === 'string' || typeof node.value === 'number')
  ) {
    return String(node.value);
  }

  return null;
}

function rejectNode(nodeType, node) {
  const line = node?.position?.start?.line;
  throw new Error(`Unsafe MDX structure rejected: ${nodeType}.${line ? ` 第 ${line} 行` : ''}`);
}

export function isSafeContentUrl(value, image = false) {
  if (typeof value !== 'string') return false;
  const url = value.trim();
  if (/[\u0000-\u0020\u007f]/.test(url) || url.startsWith('//') || url.includes('\\')) return false;
  if (/^https?:\/\//i.test(url)) return true;
  if (url.startsWith('/') || url.startsWith('#')) return true;
  if (!image && /^mailto:/i.test(url)) return true;
  return !/^[a-z][a-z0-9+.-]*:/i.test(url);
}
