const STOPWORDS: ReadonlySet<string> = new Set([
  'a',
  'an',
  'and',
  'are',
  'as',
  'at',
  'be',
  'before',
  'by',
  'does',
  'for',
  'from',
  'if',
  'in',
  'into',
  'is',
  'it',
  'of',
  'on',
  'or',
  'the',
  'this',
  'to',
  'was',
  'were',
  'will',
  'with',
]);

const SUBJECT_STOPWORDS: ReadonlySet<string> = new Set([
  'will',
  'does',
  'did',
  'has',
  'have',
  'can',
  'could',
  'would',
  'should',
  'is',
  'are',
  'was',
  'were',
  'be',
]);

const MONTH_NAMES: Readonly<Record<string, string>> = {
  january: 'january',
  jan: 'january',
  february: 'february',
  feb: 'february',
  march: 'march',
  mar: 'march',
  april: 'april',
  apr: 'april',
  may: 'may',
  june: 'june',
  jun: 'june',
  july: 'july',
  jul: 'july',
  august: 'august',
  aug: 'august',
  september: 'september',
  sep: 'september',
  sept: 'september',
  october: 'october',
  oct: 'october',
  november: 'november',
  nov: 'november',
  december: 'december',
  dec: 'december',
};

const normalizeWord = (word: string): string =>
  word.toLowerCase().replace(/[^a-z0-9]/g, '');

export const extractMonths = (text: string): Set<string> => {
  const months = new Set<string>();
  for (const word of text.toLowerCase().split(/\s+/)) {
    const normalized = normalizeWord(word);
    const canonical = MONTH_NAMES[normalized];
    if (canonical) {
      months.add(canonical);
    }
  }
  return months;
};

export const extractResolutionDates = (text: string): Set<string> => {
  const dates = new Set<string>();
  const pattern =
    /(?:by|before|on)\s+(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),?\s+(\d{4})/gi;

  for (const match of text.matchAll(pattern)) {
    dates.add(
      `${match[1].toLowerCase()} ${match[2]} ${match[3]}`,
    );
  }
  return dates;
};

export const extractSubject = (text: string): Set<string> => {
  const subject = new Set<string>();
  const trimmed = text.trim().replace(/[?.!]+$/, '');
  if (!trimmed) {
    return subject;
  }

  const words = trimmed.split(/\s+/);
  let start = 0;
  while (start < words.length) {
    if (SUBJECT_STOPWORDS.has(words[start].toLowerCase())) {
      start += 1;
      continue;
    }
    break;
  }

  for (let index = start; index < words.length; index += 1) {
    const word = words[index];
    if (word.startsWith('(') && word.endsWith(')')) {
      const inner = normalizeWord(word.slice(1, -1));
      if (inner) {
        subject.add(inner);
      }
      continue;
    }

    if (/^[a-z]/.test(word)) {
      break;
    }

    if (/^[A-Z]/.test(word)) {
      const cleaned = normalizeWord(word);
      if (cleaned && !STOPWORDS.has(cleaned)) {
        subject.add(cleaned);
        continue;
      }
    }

    break;
  }

  return subject;
};

export const setsEqual = (
  left: Set<string>,
  right: Set<string>,
): boolean => {
  if (left.size !== right.size) {
    return false;
  }
  for (const value of left) {
    if (!right.has(value)) {
      return false;
    }
  }
  return true;
};
