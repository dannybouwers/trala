import { docs, type Doc } from '../data/docs';

// Re-export Doc type for consumers
export type { Doc };

/**
 * Get page navigation data for prev/next links
 * @param currentPath - The current page path
 * @returns Object containing previous page, next page, and current index
 */
export function getPageNavigation(currentPath: string): {
  previous: Doc | null;
  next: Doc | null;
  currentIndex: number;
} {
  const currentIndex = docs.findIndex((doc) => doc.path === currentPath);
  const previous = currentIndex > 0 ? docs[currentIndex - 1] : null;
  const next = currentIndex < docs.length - 1 ? docs[currentIndex + 1] : null;

  return {
    previous,
    next,
    currentIndex,
  };
}

/**
 * Check if a given path is the current page
 * @param currentPath - The current page path
 * @param docPath - The doc path to check against
 * @returns True if the paths match
 */
export function isCurrentPage(currentPath: string, docPath: string): boolean {
  return currentPath === docPath;
}
