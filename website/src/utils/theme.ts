/**
 * Theme utility functions for centralized theme management.
 * This module provides functions to initialize, toggle, and check dark mode.
 */

const THEME_KEY = 'color-theme';

/**
 * Initialize theme based on localStorage or system preference.
 * This should be called early in the page load process.
 */
export function initTheme(): void {
  const savedTheme = localStorage.getItem(THEME_KEY);
  
  if (savedTheme) {
    document.documentElement.classList.toggle('dark', savedTheme === 'dark');
  } else {
    const theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    document.documentElement.classList.toggle('dark', theme === 'dark');
  }
}

/**
 * Toggle between light and dark mode.
 * Updates both the DOM and localStorage.
 */
export function toggleTheme(): void {
  const isDark = isDarkMode();
  document.documentElement.classList.toggle('dark', !isDark);
  localStorage.setItem(THEME_KEY, !isDark ? 'dark' : 'light');
}

/**
 * Check if dark mode is currently active.
 * @returns true if dark mode is active, false otherwise
 */
export function isDarkMode(): boolean {
  return document.documentElement.classList.contains('dark');
}
