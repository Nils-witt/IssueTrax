import { lazy, Suspense } from 'react';

// The Markdown stack (remark/micromark) is the bulk of the bundle, so it is
// loaded on first use instead of with the app shell.
const MarkdownRenderer = lazy(() => import('./MarkdownRenderer'));

export function Markdown({ children }: { children: string }) {
  return (
    <Suspense fallback={<div className="markdown">{children}</div>}>
      <MarkdownRenderer>{children}</MarkdownRenderer>
    </Suspense>
  );
}
