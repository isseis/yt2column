/**
 * The video ID of an accepted watch URL: its one non-empty v value, as the
 * URL parser decodes it. Undefined for every URL isAcceptedWatchUrl rejects:
 * both are this one parse.
 */
export function watchVideoId(url: string): string | undefined {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return undefined;
  }
  const videoIds = parsed.searchParams.getAll("v");
  const [videoId] = videoIds;
  if (
    parsed.protocol === "https:" &&
    parsed.hostname === "www.youtube.com" &&
    parsed.port === "" &&
    parsed.username === "" &&
    parsed.password === "" &&
    parsed.pathname === "/watch" &&
    videoIds.length === 1 &&
    videoId !== undefined &&
    videoId !== ""
  ) {
    return videoId;
  }
  return undefined;
}

/**
 * Reports whether url is a www.youtube.com watch page the extension collects
 * from. The parsed URL is used only for the decision; callers keep and
 * collect the original string. The value of v is not checked against the
 * video ID format: the server does that.
 */
export function isAcceptedWatchUrl(url: string): boolean {
  return watchVideoId(url) !== undefined;
}
