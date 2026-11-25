import type { Ref } from 'react';
import { Link as RouterLink, type LinkProps as RouterLinkProps } from 'react-router';

type LinkBehaviorProps = Omit<RouterLinkProps, 'to'> & { href: RouterLinkProps['to']; ref?: Ref<HTMLAnchorElement> };

/**
 * Lets MUI's Link, Button, ListItemButton, Tab, ... take an `href` and
 * navigate client-side through React Router instead of reloading the page.
 */
export function LinkBehavior({ href, ref, ...props }: LinkBehaviorProps) {
  return <RouterLink ref={ref} to={href} {...props} />;
}
