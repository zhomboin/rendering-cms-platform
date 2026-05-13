import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AppRoutes } from '../routes';
import { ErrorBoundary } from './ErrorBoundary';
import { appTheme } from './theme';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      gcTime: 15 * 60 * 1000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

export function AppProviders() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider theme={appTheme}>
        <ErrorBoundary>
          <AppRoutes />
        </ErrorBoundary>
      </ConfigProvider>
    </QueryClientProvider>
  );
}
