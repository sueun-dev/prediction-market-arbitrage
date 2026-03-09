const parseIntFlag = (value: string | undefined): number | null => {
  if (!value) {
    return null;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : null;
};

export const filterPredictArgs = (argv: string[]): string[] => {
  const filtered: string[] = [];
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg.startsWith('--full-out')) {
      if (!arg.includes('=')) {
        i += 1;
      }
      continue;
    }
    if (arg.startsWith('--out')) {
      if (!arg.includes('=')) {
        i += 1;
      }
      continue;
    }
    filtered.push(arg);
  }
  return filtered;
};

export const resolvePolymarketMaxMarkets = (
  argv: string[],
  envValue: string | undefined,
): number => {
  const envParsed = parseIntFlag(envValue);
  if (envParsed !== null) {
    return envParsed;
  }

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg.startsWith('--max-markets=')) {
      return parseIntFlag(arg.split('=', 2)[1]) ?? 0;
    }
    if (arg === '--max-markets' && i + 1 < argv.length) {
      return parseIntFlag(argv[i + 1]) ?? 0;
    }
  }

  return 0;
};
