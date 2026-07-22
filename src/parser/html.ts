/**
 * HTML parser for extracting video URLs from d3.ru pages
 * Uses native Node.js capabilities (no external libraries)
 */

/**
 * Result of video URL extraction
 */
export interface VideoExtractionResult {
  /** The video URL found in the page */
  videoUrl: string;
}

/**
 * Extract video URL from d3.ru HTML content
 * d3.ru stores page data in: window.entryStorages[window.pageName] = { ...json... }
 * Video URL is located at: post.data.media.url
 */
export function extractVideoUrl(html: string): VideoExtractionResult | null {
  try {
    // Find the start of JSON data after window.entryStorages[window.pageName] =
    const marker = "window.entryStorages[window.pageName] = ";
    const startIndex = html.indexOf(marker);

    if (startIndex === -1) {
      return null;
    }

    // Find the JSON object starting from the marker
    const jsonStart = startIndex + marker.length;

    // Find the end of JSON (look for ; at the end of the line or end of script)
    // We need to find the matching closing brace by counting braces
    let braceCount = 0;
    let jsonEnd = jsonStart;
    let foundStart = false;

    for (let i = jsonStart; i < html.length; i++) {
      const char = html[i];

      if (char === "{") {
        braceCount++;
        foundStart = true;
      } else if (char === "}") {
        braceCount--;
      }

      if (foundStart && braceCount === 0) {
        jsonEnd = i + 1;
        break;
      }
    }

    if (!foundStart || braceCount !== 0) {
      return null;
    }

    const jsonString = html.substring(jsonStart, jsonEnd);
    const jsonData = JSON.parse(jsonString);

    // Navigate to post.data.media.url
    const mediaUrl = jsonData?.post?.data?.media?.url;

    if (mediaUrl && isValidVideoUrl(mediaUrl)) {
      return { videoUrl: mediaUrl };
    }

    // Also try post.data.link.url (for video links)
    const linkUrl = jsonData?.post?.data?.link?.url;

    if (linkUrl && isValidVideoUrl(linkUrl)) {
      return { videoUrl: linkUrl };
    }

    return null;
  } catch {
    return null;
  }
}

/**
 * Check if a URL is a valid video URL (must end with .mp4)
 */
export function isValidVideoUrl(url: string): boolean {
  try {
    const urlObj = new URL(url, "https://placeholder.com");
    const pathname = urlObj.pathname.toLowerCase();
    return pathname.endsWith(".mp4");
  } catch {
    return url.toLowerCase().endsWith(".mp4");
  }
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
