declare module "can-ndjson-stream" {
  export default function ndjsonStream<T>(stream: ReadableStream<Uint8Array>): ReadableStream<T>;
}
