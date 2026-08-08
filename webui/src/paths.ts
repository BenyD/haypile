/* The daemon returns native absolute paths: /home/me on Unix,
   C:\Users\me on Windows. The UI never knows which OS it is talking to,
   but it does not need to: the shape of a path says which family it
   belongs to, so every helper here reads the path instead of probing
   the platform. */

export const isAbsolute = (p: string) => p.startsWith('/') || /^[a-zA-Z]:[\\/]/.test(p);

/* sep picks the separator the path itself uses. */
export const sep = (p: string) => (/^[a-zA-Z]:/.test(p) || p.includes('\\') ? '\\' : '/');

const lastSep = (p: string) => Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));

export const basename = (p: string) => p.slice(lastSep(p) + 1);

/* A bare drive letter is not browsable; the drive root is. */
const fixDrive = (p: string) => (/^[a-zA-Z]:$/.test(p) ? p + '\\' : p);

/* dirOf: the directory part of a typed path, for listing while the last
   segment is still being typed. */
export const dirOf = (p: string) => {
  const cut = lastSep(p);
  if (cut <= 0) return '/';
  return fixDrive(p.slice(0, cut));
};

export const parentOf = (p: string) => {
  const cut = lastSep(p);
  if (cut <= 0) return '/';
  return fixDrive(p.slice(0, cut));
};

/* trimTrailingSep strips trailing separators but never below a root
   ("/" or "C:\"). */
export const trimTrailingSep = (p: string) => {
  const trimmed = p.replace(/[\\/]+$/, '');
  if (trimmed === '') return '/';
  return fixDrive(trimmed);
};
