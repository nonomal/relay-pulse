const TRAILING_DATE_RE = /-(?:\d{8}|\d{4}-\d{2}-\d{2})$/;
const VENDOR_PREFIX_RE = /^(?:claude|gemini)-/;
const TRAILING_VERSION_RE = /-(\d+)-(\d+)$/;

export function shortenModelName(name: string): string {
  if (!name) return '';

  let s = name;
  s = s.replace(TRAILING_DATE_RE, '');
  s = s.replace(VENDOR_PREFIX_RE, '');
  s = s.replace(TRAILING_VERSION_RE, (_m, major: string, minor: string) => `-${major}.${minor}`);

  return s;
}
