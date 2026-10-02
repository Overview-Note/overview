// Shared lowlight instance used by the Tiptap code block. We register a broad
// but not exhaustive set of common languages; unknown languages fall back to
// plain text.

import { createLowlight, common } from "lowlight";

export const lowlight = createLowlight(common);
