#!/usr/bin/env node

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, extname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const markdownExtensions = new Set(['.md', '.mdx']);
const homepageSourcePath = 'src/pages/index.tsx';
const homepageCatalogPath = 'i18n/zh-Hans/code.json';

const contentChecks = [
  {
    label: 'Docs',
    source: 'docs',
    translation:
      'i18n/zh-Hans/docusaurus-plugin-content-docs/current',
  },
  {
    label: 'Blog',
    source: 'blog',
    translation: 'i18n/zh-Hans/docusaurus-plugin-content-blog',
  },
];

const jsonCatalogPaths = [
  homepageCatalogPath,
  'i18n/zh-Hans/docusaurus-theme-classic/navbar.json',
  'i18n/zh-Hans/docusaurus-theme-classic/footer.json',
  'i18n/zh-Hans/docusaurus-plugin-content-docs/current.json',
  'i18n/zh-Hans/docusaurus-plugin-content-blog/options.json',
];

const requiredFiles = [
  ...jsonCatalogPaths,
  'blog/tags.yml',
  'i18n/zh-Hans/docusaurus-plugin-content-blog/authors.yml',
  'i18n/zh-Hans/docusaurus-plugin-content-blog/tags.yml',
].sort(compareStrings);

const errors = [];
const summaries = [];

for (const check of contentChecks) {
  const sourceFiles = collectMarkdownFiles(resolve(siteRoot, check.source));
  const translatedFiles = collectMarkdownFiles(
    resolve(siteRoot, check.translation),
  );
  const missing = difference(sourceFiles, translatedFiles);
  const extra = difference(translatedFiles, sourceFiles);

  if (missing.length === 0 && extra.length === 0) {
    summaries.push(`${check.label}: ${sourceFiles.length} file(s) matched.`);
  } else {
    errors.push(
      [
        `${check.label} Markdown file-set mismatch:`,
        ...formatList(
          'Missing translations',
          missing.map((relativePath) => `${check.translation}/${relativePath}`),
        ),
        ...formatList(
          'Extra translations',
          extra.map((relativePath) => `${check.translation}/${relativePath}`),
        ),
      ].join('\n'),
    );
  }

  const translatedFileSet = new Set(translatedFiles);
  for (const relativePath of sourceFiles) {
    if (translatedFileSet.has(relativePath)) {
      validateMarkdownPair(check, relativePath);
    }
  }
}

const requiredFileContents = validateRequiredFiles(requiredFiles);
const jsonCatalogs = validateJsonCatalogs(
  jsonCatalogPaths,
  requiredFileContents,
);
validateHomepageCatalog(jsonCatalogs.get(homepageCatalogPath));

if (errors.length > 0) {
  console.error(errors.join('\n\n'));
  process.exitCode = 1;
} else {
  for (const summary of summaries) {
    console.log(summary);
  }
  console.log('i18n content completeness check passed.');
}

function validateMarkdownPair(check, relativePath) {
  const sourceRelativePath = `${check.source}/${relativePath}`;
  const translationRelativePath = `${check.translation}/${relativePath}`;
  const sourceHeadings = readMarkdownHeadings(sourceRelativePath);
  const translatedHeadings = readMarkdownHeadings(translationRelativePath);

  if (sourceHeadings === undefined || translatedHeadings === undefined) {
    return;
  }

  validateHeadingIds(sourceRelativePath, sourceHeadings);
  validateHeadingIds(translationRelativePath, translatedHeadings);

  const sourceIds = sourceHeadings.map((heading) => heading.id);
  const translatedIds = translatedHeadings.map((heading) => heading.id);
  const mismatchIndex = firstMismatchIndex(sourceIds, translatedIds);

  if (mismatchIndex === -1) {
    return;
  }

  errors.push(
    [
      `${check.label} heading ID sequence mismatch for ${relativePath}:`,
      `  Source file: ${sourceRelativePath}`,
      `  Translation file: ${translationRelativePath}`,
      `  First mismatch at heading ${mismatchIndex + 1}:`,
      `    Source: ${formatHeadingAt(sourceHeadings, mismatchIndex)}`,
      `    Translation: ${formatHeadingAt(translatedHeadings, mismatchIndex)}`,
      `  Source IDs: ${formatHeadingSequence(sourceHeadings)}`,
      `  Translation IDs: ${formatHeadingSequence(translatedHeadings)}`,
    ].join('\n'),
  );
}

function readMarkdownHeadings(relativePath) {
  let content;

  try {
    content = readFileSync(resolve(siteRoot, relativePath), 'utf8');
  } catch (error) {
    errors.push(
      `${relativePath}: unable to read Markdown file: ${getErrorMessage(error)}`,
    );
    return undefined;
  }

  return extractMarkdownHeadings(content);
}

function extractMarkdownHeadings(content) {
  const lines = content.replace(/^\uFEFF/, '').split(/\r?\n/);
  const headings = [];
  let fence;
  let inFrontMatter = lines[0]?.trim() === '---';

  for (const [index, line] of lines.entries()) {
    if (inFrontMatter) {
      if (index > 0 && /^(?:---|\.\.\.)[ \t]*$/.test(line)) {
        inFrontMatter = false;
      }
      continue;
    }

    if (fence !== undefined) {
      const closingFence = line.match(/^ {0,3}(`+|~+)[ \t]*$/);
      if (
        closingFence !== null &&
        closingFence[1][0] === fence.marker &&
        closingFence[1].length >= fence.length
      ) {
        fence = undefined;
      }
      continue;
    }

    const openingFence = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (openingFence !== null) {
      fence = {
        marker: openingFence[1][0],
        length: openingFence[1].length,
      };
      continue;
    }

    const heading = line.match(/^ {0,3}(#{1,6})(?:[ \t]+|$)(.*)$/);
    if (heading === null) {
      continue;
    }

    const body = heading[2];
    const explicitId = body.match(
      /(?:^|[ \t])\{#([^{}\s]+)\}[ \t]*(?:#+[ \t]*)?$/,
    );
    const text = body
      .replace(
        /(?:^|[ \t])\{#[^{}\s]+\}[ \t]*(?:#+[ \t]*)?$/,
        '',
      )
      .replace(/[ \t]+#+[ \t]*$/, '')
      .trim();

    headings.push({
      id: explicitId?.[1],
      line: index + 1,
      text,
    });
  }

  return headings;
}

function validateHeadingIds(relativePath, headings) {
  const linesById = new Map();

  for (const heading of headings) {
    if (heading.id === undefined) {
      errors.push(
        `${relativePath}:${heading.line}: heading ${JSON.stringify(
          heading.text,
        )} is missing an explicit {#id}.`,
      );
      continue;
    }

    const lines = linesById.get(heading.id) ?? [];
    lines.push(heading.line);
    linesById.set(heading.id, lines);
  }

  for (const [id, lines] of linesById) {
    if (lines.length > 1) {
      errors.push(
        `${relativePath}: duplicate heading ID ${JSON.stringify(
          id,
        )} on lines ${lines.join(', ')}.`,
      );
    }
  }
}

function firstMismatchIndex(left, right) {
  const length = Math.max(left.length, right.length);
  for (let index = 0; index < length; index += 1) {
    if (left[index] !== right[index]) {
      return index;
    }
  }
  return -1;
}

function formatHeadingAt(headings, index) {
  const heading = headings[index];
  if (heading === undefined) {
    return '(no heading)';
  }
  if (heading.id === undefined) {
    return `(missing ID, line ${heading.line})`;
  }
  return `${JSON.stringify(heading.id)} (line ${heading.line})`;
}

function formatHeadingSequence(headings) {
  if (headings.length === 0) {
    return '(none)';
  }
  return headings
    .map((heading) =>
      heading.id === undefined
        ? `<missing ID at line ${heading.line}>`
        : heading.id,
    )
    .join(' -> ');
}

function validateRequiredFiles(relativePaths) {
  const contents = new Map();

  for (const relativePath of relativePaths) {
    const absolutePath = resolve(siteRoot, relativePath);
    if (!isFile(absolutePath)) {
      errors.push(
        `${relativePath}: required file is missing or is not a regular file.`,
      );
      continue;
    }

    let content;
    try {
      content = readFileSync(absolutePath, 'utf8');
    } catch (error) {
      errors.push(
        `${relativePath}: unable to read required file: ${getErrorMessage(
          error,
        )}`,
      );
      continue;
    }

    if (content.trim().length === 0) {
      errors.push(`${relativePath}: required file is empty.`);
      continue;
    }

    contents.set(relativePath, content);
  }

  return contents;
}

function validateJsonCatalogs(relativePaths, contents) {
  const catalogs = new Map();

  for (const relativePath of [...relativePaths].sort(compareStrings)) {
    const content = contents.get(relativePath);
    if (content === undefined) {
      continue;
    }

    let catalog;
    try {
      catalog = JSON.parse(content);
    } catch (error) {
      errors.push(
        `${relativePath}: invalid JSON catalog: ${getErrorMessage(error)}`,
      );
      continue;
    }

    if (!isObject(catalog)) {
      errors.push(`${relativePath}: JSON catalog must be a non-array object.`);
      continue;
    }

    catalogs.set(relativePath, catalog);
    const entries = Object.entries(catalog).sort(([left], [right]) =>
      compareStrings(left, right),
    );

    if (entries.length === 0) {
      errors.push(`${relativePath}: JSON catalog must contain entries.`);
      continue;
    }

    for (const [id, entry] of entries) {
      if (!isObject(entry)) {
        errors.push(
          `${relativePath}: catalog entry ${JSON.stringify(
            id,
          )} must be a message object.`,
        );
      } else if (
        typeof entry.message !== 'string' ||
        entry.message.trim().length === 0
      ) {
        errors.push(
          `${relativePath}: catalog entry ${JSON.stringify(
            id,
          )} must have a non-empty string "message".`,
        );
      }
    }
  }

  return catalogs;
}

function validateHomepageCatalog(catalog) {
  let source;
  try {
    source = readFileSync(resolve(siteRoot, homepageSourcePath), 'utf8');
  } catch (error) {
    errors.push(
      `${homepageSourcePath}: unable to read homepage source: ${getErrorMessage(
        error,
      )}`,
    );
    return;
  }

  if (catalog === undefined) {
    return;
  }

  const sourceIds = extractHomepageTranslationIds(source);
  const catalogIds = Object.keys(catalog)
    .filter((id) => id.startsWith('homepage.'))
    .sort(compareStrings);
  const missing = difference(sourceIds, catalogIds);
  const extra = difference(catalogIds, sourceIds);

  if (missing.length === 0 && extra.length === 0) {
    return;
  }

  errors.push(
    [
      'Homepage translation ID set mismatch:',
      `  Source file: ${homepageSourcePath}`,
      `  Catalog file: ${homepageCatalogPath}`,
      ...formatList('Missing catalog entries', missing),
      ...formatList('Extra catalog entries', extra),
    ].join('\n'),
  );
}

function extractHomepageTranslationIds(source) {
  const ids = new Set();
  const patterns = [
    /<Translate\b[^>]*\bid\s*=\s*(?:"(homepage\.[A-Za-z0-9_.-]+)"|'(homepage\.[A-Za-z0-9_.-]+)'|\{\s*"(homepage\.[A-Za-z0-9_.-]+)"\s*\}|\{\s*'(homepage\.[A-Za-z0-9_.-]+)'\s*\})/g,
    /\btranslate\s*\(\s*\{\s*id\s*:\s*(?:"(homepage\.[A-Za-z0-9_.-]+)"|'(homepage\.[A-Za-z0-9_.-]+)'|`(homepage\.[A-Za-z0-9_.-]+)`)/g,
  ];

  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      const id = match.slice(1).find((value) => value !== undefined);
      if (id !== undefined) {
        ids.add(id);
      }
    }
  }

  return [...ids].sort(compareStrings);
}

function collectMarkdownFiles(root) {
  const files = [];

  if (!isDirectory(root)) {
    return files;
  }

  walk(root, '');
  return files.sort(compareStrings);

  function walk(directory, relativeDirectory) {
    const entries = readdirSync(directory, { withFileTypes: true }).sort(
      (left, right) => compareStrings(left.name, right.name),
    );

    for (const entry of entries) {
      const relativePath = relativeDirectory
        ? `${relativeDirectory}/${entry.name}`
        : entry.name;
      const absolutePath = resolve(directory, entry.name);

      if (entry.isDirectory()) {
        walk(absolutePath, relativePath);
      } else if (
        entry.isFile() &&
        markdownExtensions.has(extname(entry.name))
      ) {
        files.push(relativePath);
      }
    }
  }
}

function difference(left, right) {
  const rightSet = new Set(right);
  return left.filter((item) => !rightSet.has(item));
}

function formatList(label, items) {
  const lines = [`  ${label}:`];
  if (items.length === 0) {
    lines.push('    (none)');
    return lines;
  }

  for (const item of items) {
    lines.push(`    - ${item}`);
  }
  return lines;
}

function compareStrings(left, right) {
  return left < right ? -1 : left > right ? 1 : 0;
}

function isObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function getErrorMessage(error) {
  return error instanceof Error ? error.message : String(error);
}

function isDirectory(path) {
  try {
    return statSync(path).isDirectory();
  } catch {
    return false;
  }
}

function isFile(path) {
  try {
    return statSync(path).isFile();
  } catch {
    return false;
  }
}
