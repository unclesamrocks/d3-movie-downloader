/**
 * HTML parser for extracting video URLs from web pages
 * Uses native Node.js capabilities (no external libraries)
 */

/**
 * Result of video URL extraction
 */
export interface VideoExtractionResult {
  /** The video URL found in the page */
  videoUrl: string;
  /** The source element attributes (if any) */
  sourceAttributes?: Record<string, string>;
  /** The video element attributes (if any) */
  videoAttributes?: Record<string, string>;
}

/**
 * Extract video URL from HTML content
 * Looks for <video> elements and their nested <source> elements
 * Only returns .mp4 URLs
 */
export function extractVideoUrl(html: string): VideoExtractionResult | null {
  // First, try to find <source> elements inside <video> elements
  const sourceMatch = html.match(/<video[^>]*>[\s\S]*?<source[^>]+src=["']([^"']+)["'][^>]*>[\s\S]*?<\/video>/i);

  if (sourceMatch && sourceMatch[1]) {
    const url = sourceMatch[1];
    if (isValidVideoUrl(url)) {
      return {
        videoUrl: url,
        sourceAttributes: extractAttributes(sourceMatch[0]),
        videoAttributes: extractAttributes(sourceMatch[0].match(/<video[^>]*>/)?.[0] || ''),
      };
    }
  }

  // Try to find <video> elements with src attribute directly
  const videoSrcMatch = html.match(/<video[^>]+src=["']([^"']+)["'][^>]*>/i);

  if (videoSrcMatch && videoSrcMatch[1]) {
    const url = videoSrcMatch[1];
    if (isValidVideoUrl(url)) {
      return {
        videoUrl: url,
        videoAttributes: extractAttributes(videoSrcMatch[0]),
      };
    }
  }

  // Try to find any <source> element with mp4
  const anySourceMatch = html.match(/<source[^>]+src=["']([^"']+\.mp4[^"']*)["'][^>]*>/i);

  if (anySourceMatch && anySourceMatch[1]) {
    return {
      videoUrl: anySourceMatch[1],
      sourceAttributes: extractAttributes(anySourceMatch[0]),
    };
  }

  // Try to find any video URL ending in .mp4
  const mp4Match = html.match(/["']([^"']+\.mp4[^"']*)["']/i);

  if (mp4Match && mp4Match[1]) {
    return {
      videoUrl: mp4Match[1],
    };
  }

  return null;
}

/**
 * Check if a URL is a valid video URL (must end with .mp4)
 */
export function isValidVideoUrl(url: string): boolean {
  // Must be a valid URL or path
  try {
    // Handle relative URLs by checking extension
    const urlObj = new URL(url, 'https://placeholder.com');
    const pathname = urlObj.pathname.toLowerCase();
    return pathname.endsWith('.mp4');
  } catch {
    // If URL parsing fails, check if it ends with .mp4
    return url.toLowerCase().endsWith('.mp4');
  }
}

/**
 * Extract attributes from an HTML tag
 */
function extractAttributes(tag: string): Record<string, string> {
  const attributes: Record<string, string> = {};
  const attrRegex = /(\w+)=["']([^"']*)["']/g;
  let match;

  while ((match = attrRegex.exec(tag)) !== null) {
    attributes[match[1]] = match[2];
  }

  return attributes;
}

/**
 * Resolve a potentially relative URL against a base URL
 */
export function resolveUrl(baseUrl: string, videoUrl: string): string {
  try {
    return new URL(videoUrl, baseUrl).href;
  } catch {
    return videoUrl;
  }
}

/**
 * Extract the first video URL from HTML, resolving relative URLs
 * @param html The HTML content
 * @param baseUrl The base URL for resolving relative paths
 * @returns The resolved video URL or null
 */
export function findVideoUrl(html: string, baseUrl: string): string | null {
  const result = extractVideoUrl(html);

  if (!result) {
    return null;
  }

  // Resolve relative URLs
  const resolvedUrl = resolveUrl(baseUrl, result.videoUrl);

  // Verify it's still an mp4
  if (!isValidVideoUrl(resolvedUrl)) {
    return null;
  }

  return resolvedUrl;
}