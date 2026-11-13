import { useTranslation } from 'react-i18next';

type AppProps = {
  version: string;
};

export function App({ version }: AppProps) {
  const { t } = useTranslation();

  return (
    <main>
      <h1>Intársia</h1>
      <p>{t('home.tagline')}</p>
      <p>{t('home.version', { version })}</p>
    </main>
  );
}
