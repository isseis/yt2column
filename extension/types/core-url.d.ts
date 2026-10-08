// The WHATWG URL API that src/core/ may use. tsconfig.core.json checks
// src/core/ without the DOM library, so that a reference to document or
// chrome fails the check; URL is declared only there, and only in the DOM
// library and @types/node. This file declares the members src/core/ uses and
// is read by tsconfig.core.json alone: the other configurations get the full
// declarations from the DOM library.

declare class URLSearchParams {
  getAll(name: string): string[];
}

declare class URL {
  constructor(url: string);
  readonly protocol: string;
  readonly username: string;
  readonly password: string;
  readonly hostname: string;
  readonly port: string;
  readonly pathname: string;
  readonly searchParams: URLSearchParams;
}
