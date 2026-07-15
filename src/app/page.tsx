import { RootAuthFlow } from '@/components/auth/RootAuthFlow';
import { PublicHome } from '@/components/marketing/PublicHome';

type SearchParams = Record<string, string | string[] | undefined>;

function firstValue(value: string | string[] | undefined): string {
  return Array.isArray(value) ? value[0] || '' : value || '';
}

export default async function Home({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const params = await searchParams;

  if (Object.prototype.hasOwnProperty.call(params, 'verifyEmailToken')) {
    const token = firstValue(params.verifyEmailToken);
    return <RootAuthFlow key={`verify:${token}`} kind="verify" token={token} />;
  }
  if (Object.prototype.hasOwnProperty.call(params, 'resetPasswordToken')) {
    const token = firstValue(params.resetPasswordToken);
    return <RootAuthFlow key={`reset:${token}`} kind="reset" token={token} />;
  }
  return <PublicHome />;
}
