import Box from '@mui/material/Box';
import type { ReactNode } from 'react';

/** A project or issue key such as `ABC` or `ABC-12`, set in monospace. */
export function KeyText({ children }: { children: ReactNode }) {
  return (
    <Box component="span" sx={{ fontFamily: 'monospace', fontWeight: 600, color: 'text.secondary', whiteSpace: 'nowrap' }}>
      {children}
    </Box>
  );
}
