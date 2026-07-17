export const WEB_BUILD_VERSION = process.env.NEXT_PUBLIC_BUILD_VERSION || 'local';
export const WEB_BUILD_COMMIT = process.env.NEXT_PUBLIC_BUILD_COMMIT || 'local';

export function reloadForBuild(version: string) {
  const url = new URL(window.location.href);
  url.searchParams.set('_build', version || String(Date.now()));
  window.location.replace(url.toString());
}
