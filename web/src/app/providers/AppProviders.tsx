import type { PropsWithChildren } from 'react';
import { useState } from 'react';
import { ConfigProvider, theme } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            refetchOnWindowFocus: false,
            retry: 1,
          },
          mutations: {
            retry: 0,
          },
        },
      }),
  );

  return (
    <ConfigProvider
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorBgBase: '#09111d',
          colorBgContainer: '#121c2a',
          colorBorder: '#263447',
          colorPrimary: '#45d6ff',
          colorText: '#edf3ff',
          colorTextSecondary: '#8fa7c4',
          colorInfo: '#45d6ff',
          colorSuccess: '#38d39f',
          colorWarning: '#ffbe5c',
          borderRadius: 18,
          fontFamily: '"IBM Plex Sans", "Aptos", "Segoe UI", sans-serif',
        },
      }}
    >
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ConfigProvider>
  );
}
