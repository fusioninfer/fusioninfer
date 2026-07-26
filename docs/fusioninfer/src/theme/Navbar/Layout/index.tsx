import React, {
  type ComponentProps,
  type ReactNode,
  useEffect,
  useRef,
} from 'react';
import {useNavbarMobileSidebar} from '@docusaurus/theme-common/internal';
import OriginalNavbarLayout from '@theme-original/Navbar/Layout';

type Props = ComponentProps<typeof OriginalNavbarLayout>;

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

function getVisibleFocusableElements(container: Element): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>(focusableSelector),
  ).filter((element) => {
    const style = window.getComputedStyle(element);
    const rect = element.getBoundingClientRect();

    return (
      style.display !== 'none' &&
      style.visibility !== 'hidden' &&
      !element.closest('[inert]') &&
      rect.width > 0 &&
      rect.height > 0 &&
      rect.right > 0 &&
      rect.bottom > 0 &&
      rect.left < window.innerWidth &&
      rect.top < window.innerHeight
    );
  });
}

export default function NavbarLayout(props: Props): ReactNode {
  const mobileSidebar = useNavbarMobileSidebar();
  const triggerRef = useRef<HTMLElement | null>(null);
  const wasShownRef = useRef(false);

  useEffect(() => {
    const rememberTrigger = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }

      const trigger = target.closest<HTMLElement>('.navbar__toggle');
      if (trigger) {
        triggerRef.current = trigger;
      }
    };

    document.addEventListener('click', rememberTrigger, true);
    return () => document.removeEventListener('click', rememberTrigger, true);
  }, []);

  useEffect(() => {
    const wasShown = wasShownRef.current;
    wasShownRef.current = mobileSidebar.shown;

    if (mobileSidebar.shown && !wasShown) {
      let frame = 0;
      let attemptsRemaining = 60;

      const focusFirstSidebarControl = () => {
        const sidebar = document.querySelector('.navbar-sidebar');
        const firstFocusable =
          sidebar?.querySelector<HTMLElement>('.navbar__brand') ??
          (sidebar ? getVisibleFocusableElements(sidebar)[0] : undefined);
        if (firstFocusable) {
          firstFocusable.focus({preventScroll: true});
          if (document.activeElement === firstFocusable) {
            return;
          }
        }

        attemptsRemaining -= 1;
        if (attemptsRemaining > 0) {
          frame = window.requestAnimationFrame(focusFirstSidebarControl);
        }
      };

      frame = window.requestAnimationFrame(focusFirstSidebarControl);

      return () => window.cancelAnimationFrame(frame);
    }

    if (!mobileSidebar.shown && wasShown) {
      const frame = window.requestAnimationFrame(() => {
        triggerRef.current?.focus();
      });

      return () => window.cancelAnimationFrame(frame);
    }

    return undefined;
  }, [mobileSidebar.shown]);

  useEffect(() => {
    if (!mobileSidebar.shown) {
      return undefined;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        mobileSidebar.toggle();
        return;
      }

      if (event.key !== 'Tab') {
        return;
      }

      const sidebar = document.querySelector('.navbar-sidebar');
      if (!sidebar) {
        return;
      }

      const focusable = getVisibleFocusableElements(sidebar);
      const first = focusable[0];
      const last = focusable.at(-1);
      if (!first || !last) {
        return;
      }

      const active = document.activeElement;
      if (event.shiftKey && (active === first || !sidebar.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (
        !event.shiftKey &&
        (active === last || !sidebar.contains(active))
      ) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [mobileSidebar]);

  return <OriginalNavbarLayout {...props} />;
}
