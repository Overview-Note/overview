export interface CompressOptions {
  maxWidth: number;
  maxHeight: number;
  quality: number;
  maxBytes: number;
}

export const defaultCompressOptions: CompressOptions = {
  maxWidth: 1920,
  maxHeight: 1920,
  quality: 0.82,
  maxBytes: 2 * 1024 * 1024,
};

const COMPRESSIBLE = new Set(["image/jpeg", "image/png", "image/webp", "image/bmp"]);

/**
 * Compresses an image file in the browser by downscaling it to fit within
 * maxWidth/maxHeight and re-encoding. Animated GIFs, SVGs and files already
 * within limits are returned untouched. Falls back to the original file on
 * any failure so uploads never break.
 */
export async function compressImage(
  file: File,
  options: Partial<CompressOptions> = {},
): Promise<File> {
  const opts = { ...defaultCompressOptions, ...options };
  if (!COMPRESSIBLE.has(file.type)) return file;
  // Never touch animated GIFs (canvas would drop the animation).
  if (file.type === "image/gif") return file;

  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    return file;
  }

  const { width, height } = bitmap;
  const scale = Math.min(1, opts.maxWidth / width, opts.maxHeight / height);
  const targetW = Math.max(1, Math.round(width * scale));
  const targetH = Math.max(1, Math.round(height * scale));

  // If no resize is needed and the file is already small, keep it as-is.
  if (scale === 1 && file.size <= opts.maxBytes) {
    bitmap.close();
    return file;
  }

  const canvas = document.createElement("canvas");
  canvas.width = targetW;
  canvas.height = targetH;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    bitmap.close();
    return file;
  }
  ctx.drawImage(bitmap, 0, 0, targetW, targetH);
  bitmap.close();

  // Prefer WebP when the browser can encode it, else JPEG.
  const outputType = supportsWebp(canvas) ? "image/webp" : "image/jpeg";
  let quality = opts.quality;
  let blob = await toBlob(canvas, outputType, quality);
  // Progressively lower quality until under the byte budget.
  for (let i = 0; i < 4 && blob && blob.size > opts.maxBytes; i++) {
    quality = Math.max(0.4, quality - 0.15);
    blob = await toBlob(canvas, outputType, quality);
  }
  if (!blob || blob.size >= file.size) return file;

  const ext = outputType === "image/webp" ? ".webp" : ".jpg";
  const name = renameExt(file.name, ext);
  return new File([blob], name, { type: outputType, lastModified: Date.now() });
}

function toBlob(
  canvas: HTMLCanvasElement,
  type: string,
  quality: number,
): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

let webpSupport: boolean | null = null;
function supportsWebp(canvas: HTMLCanvasElement): boolean {
  if (webpSupport === null) {
    webpSupport = canvas.toDataURL("image/webp").startsWith("data:image/webp");
  }
  return webpSupport;
}

function renameExt(name: string, ext: string): string {
  const dot = name.lastIndexOf(".");
  const base = dot > 0 ? name.slice(0, dot) : name;
  return base + ext;
}
