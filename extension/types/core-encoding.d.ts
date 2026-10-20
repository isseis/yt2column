// The TextEncoder, TextDecoder, atob and btoa APIs that src/core/ may use.
// tsconfig.core.json checks src/core/ without the DOM library, so that a
// reference to document or chrome fails the check; these are declared only in
// the DOM library and @types/node. This file declares the members src/core/ uses and
// is read by tsconfig.core.json alone: the other configurations get the full
// declarations from the DOM library.

interface TextDecoderOptions {
  fatal?: boolean;
  ignoreBOM?: boolean;
}

declare class TextEncoder {
  encode(input?: string): Uint8Array;
}

declare class TextDecoder {
  constructor(label?: string, options?: TextDecoderOptions);
  decode(input?: Uint8Array): string;
}

declare function atob(data: string): string;
declare function btoa(data: string): string;
