import { describe, expect, it } from 'vitest';
import { basename, dirOf, isAbsolute, parentOf, sep, trimTrailingSep } from './paths';

/* The daemon hands the UI native paths from either OS; every helper must
   read the path's own shape rather than assume the server's platform. */
describe('paths', () => {
  it('recognises absolute paths from both families', () => {
    expect(isAbsolute('/home/me')).toBe(true);
    expect(isAbsolute('C:\\Users\\me')).toBe(true);
    expect(isAbsolute('c:/Users/me')).toBe(true);
    expect(isAbsolute('Documents')).toBe(false);
    expect(isAbsolute('~/Documents')).toBe(false);
  });

  it('picks the separator the path uses', () => {
    expect(sep('/home/me')).toBe('/');
    expect(sep('C:\\Users\\me')).toBe('\\');
    expect(sep('C:')).toBe('\\');
  });

  it('takes basenames across separators', () => {
    expect(basename('/home/me/q3.pdf')).toBe('q3.pdf');
    expect(basename('C:\\Users\\me\\q3.pdf')).toBe('q3.pdf');
  });

  it('finds the directory of a partly typed path', () => {
    expect(dirOf('/home/m')).toBe('/home');
    expect(dirOf('/h')).toBe('/');
    expect(dirOf('C:\\Users\\m')).toBe('C:\\Users');
    expect(dirOf('C:\\U')).toBe('C:\\');
  });

  it('walks to a parent without leaving the drive', () => {
    expect(parentOf('/home/me')).toBe('/home');
    expect(parentOf('C:\\Users')).toBe('C:\\');
  });

  it('trims trailing separators but never a root', () => {
    expect(trimTrailingSep('/home/me/')).toBe('/home/me');
    expect(trimTrailingSep('/')).toBe('/');
    expect(trimTrailingSep('C:\\Users\\')).toBe('C:\\Users');
    expect(trimTrailingSep('C:\\')).toBe('C:\\');
  });
});
