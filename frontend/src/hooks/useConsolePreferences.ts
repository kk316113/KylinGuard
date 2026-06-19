"use client";

import { useCallback, useEffect, useState } from "react";

export type ConsoleTheme = "system" | "light" | "dark";
export type ChatPosition = "left" | "right";

export type ConsolePreferences = {
  theme: ConsoleTheme;
  chatPosition: ChatPosition;
  chatWidth: number;
  chatDefaultOpen: boolean;
};

const STORAGE_KEY = "kylinguard.console.preferences.v1";
const MIN_CHAT_WIDTH = 360;
const MAX_CHAT_WIDTH = 640;

export const defaultConsolePreferences: ConsolePreferences = {
  theme: "system",
  chatPosition: "right",
  chatWidth: 480,
  chatDefaultOpen: false,
};

function normalizePreferences(value: unknown): ConsolePreferences {
  if (!value || typeof value !== "object") {
    return defaultConsolePreferences;
  }

  const candidate = value as Partial<ConsolePreferences>;
  const theme = candidate.theme === "light" || candidate.theme === "dark" ? candidate.theme : "system";
  const chatPosition = candidate.chatPosition === "left" ? "left" : "right";
  const chatWidth = Math.min(
    MAX_CHAT_WIDTH,
    Math.max(MIN_CHAT_WIDTH, Number.isFinite(candidate.chatWidth) ? Number(candidate.chatWidth) : 480),
  );

  return {
    theme,
    chatPosition,
    chatWidth,
    chatDefaultOpen: candidate.chatDefaultOpen === true,
  };
}

export function useConsolePreferences() {
  const [preferences, setPreferences] = useState<ConsolePreferences>(defaultConsolePreferences);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (stored) {
        setPreferences(normalizePreferences(JSON.parse(stored)));
      }
    } catch {
      window.localStorage.removeItem(STORAGE_KEY);
    } finally {
      setHydrated(true);
    }
  }, []);

  useEffect(() => {
    if (!hydrated) {
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
  }, [hydrated, preferences]);

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const applyTheme = () => {
      const dark = preferences.theme === "dark" || (preferences.theme === "system" && media.matches);
      document.documentElement.classList.toggle("dark", dark);
      document.documentElement.style.colorScheme = dark ? "dark" : "light";
    };

    applyTheme();
    media.addEventListener("change", applyTheme);
    return () => media.removeEventListener("change", applyTheme);
  }, [preferences.theme]);

  const updatePreferences = useCallback((patch: Partial<ConsolePreferences>) => {
    setPreferences((current) => normalizePreferences({ ...current, ...patch }));
  }, []);

  const resetPreferences = useCallback(() => {
    setPreferences(defaultConsolePreferences);
  }, []);

  return { preferences, updatePreferences, resetPreferences, hydrated };
}
