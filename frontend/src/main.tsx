import CssBaseline from '@mui/material/CssBaseline';
import { ThemeProvider } from '@mui/material/styles';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter } from 'react-router';
import { RouterProvider } from 'react-router/dom';
import { ApiError } from './api/client';
import { ErrorPage, NotFoundPage } from './app/ErrorPage';
import { Layout } from './app/Layout';
import './i18n';
import { IssuePage } from './pages/IssuePage';
import { IssuesPage } from './pages/IssuesPage';
import { NewIssuePage } from './pages/NewIssuePage';
import { NewProjectPage } from './pages/NewProjectPage';
import { ProjectSettingsPage } from './pages/ProjectSettingsPage';
import { ProjectsPage } from './pages/ProjectsPage';
import { theme } from './theme';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      // Client errors (404, 422, ...) will not go away by retrying.
      retry: (failures, error) => !(error instanceof ApiError && error.status >= 400 && error.status < 500) && failures < 2,
    },
  },
});

const router = createBrowserRouter([
  {
    element: <Layout />,
    errorElement: <ErrorPage />,
    children: [
      { index: true, element: <ProjectsPage /> },
      { path: 'projects/new', element: <NewProjectPage /> },
      { path: 'p/:key', element: <IssuesPage /> },
      { path: 'p/:key/issues/new', element: <NewIssuePage /> },
      { path: 'p/:key/issues/:number', element: <IssuePage /> },
      { path: 'p/:key/settings', element: <ProjectSettingsPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme} defaultMode="system">
      <CssBaseline enableColorScheme />
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
);
