/**
 * Reports whether url is a www.youtube.com watch page the extension collects
 * from. The parsed URL is used only for the decision; callers keep and
 * collect the original string. The value of v is not checked against the
 * video ID format: the server does that.
 */
export function isAcceptedWatchUrl( url: string ): boolean {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  const videoIds = parsed.searchParams.getAll("v");
  return (
    parsed.protocol === "https:" &&
    parsed.hostname === "www.youtube.com" &&
    parsed.port === "" &&
    parsed.username === "" &&
    parsed.password === "" &&
    parsed.pathname === "/watch" &&
    videoIds.length === 1 &&
    videoIds[0] !== ""
  );
}
