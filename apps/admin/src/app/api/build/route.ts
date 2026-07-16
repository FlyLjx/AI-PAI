export const dynamic = 'force-dynamic';

export function GET() {
  return Response.json({
    version: process.env.NEXT_PUBLIC_BUILD_VERSION || 'local',
    commit: process.env.NEXT_PUBLIC_BUILD_COMMIT || 'local',
  }, {
    headers: {
      'Cache-Control': 'private, no-cache, no-store, max-age=0, must-revalidate',
    },
  });
}
