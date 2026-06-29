"use client";

import { useCallback, useEffect, useState } from "react";

export type ConsoleTheme = "system" | "light" | "dark";
export type ChatPosition = "left" | "center" | "right";
export type GlassIntensity = "soft" | "balanced" | "strong";

export type ConsolePreferences = {
  theme: ConsoleTheme;
  chatPosition: ChatPosition;
  chatWidth: number;
  chatHeight: number;
  chatDefaultOpen: boolean;
  glassIntensity: GlassIntensity;
  reduceMotion: boolean;
  compactMode: boolean;
  processPanelDefaultOpen: boolean;
  attachmentsEnabled: boolean;
};

const STORAGE_KEY = "kylinguard.console.preferences.v1";
const DEFAULT_CHAT_WIDTH = 860;
const DEFAULT_CHAT_HEIGHT = 680;
const MIN_CHAT_WIDTH = 560;
const MAX_CHAT_WIDTH = 1100;
const MIN_CHAT_HEIGHT = 430;
const MAX_CHAT_HEIGHT = 820;

export const defaultConsolePreferences: ConsolePreferences = {
  theme: "system",
  chatPosition: "center",
  chatWidth: DEFAULT_CHAT_WIDTH,
  chatHeight: DEFAULT_CHAT_HEIGHT,
  chatDefaultOpen: false,
  glassIntensity: "balanced",
  reduceMotion: false,
  compactMode: false,
  processPanelDefaultOpen: true,
  attachmentsEnabled: true,
};

function normalizePreferences(value: unknown): ConsolePreferences {
  if (!value || typeof value !== "object") {
    return defaultConsolePreferences;
  }

  const candidate = value as Partial<ConsolePreferences>;
  const theme = candidate.theme === "light" || candidate.theme === "dark" ? candidate.theme : "system";
  const chatPosition =
    candidate.chatPosition === "left" || candidate.chatPosition === "right" || candidate.chatPosition === "center"
      ? candidate.chatPosition
      : "center";
  const chatWidth = Math.min(
    MAX_CHAT_WIDTH,
    Math.max(
      MIN_CHAT_WIDTH,
      Number.isFinite(candidate.chatWidth) ? Number(candidate.chatWidth) : DEFAULT_CHAT_WIDTH,
    ),
  );
  const chatHeight = Math.min(
    MAX_CHAT_HEIGHT,
    Math.max(
      MIN_CHAT_HEIGHT,
      Number.isFinite(candidate.chatHeight) ? Number(candidate.chatHeight) : DEFAULT_CHAT_HEIGHT,
    ),
  );
  const glassIntensity =
    candidate.glassIntensity === "soft" || candidate.glassIntensity === "strong"
      ? candidate.glassIntensity
      : "balanced";

  return {
    theme,
    chatPosition,
    chatWidth,
    chatHeight,
    chatDefaultOpen: candidate.chatDefaultOpen === true,
    glassIntensity,
    reduceMotion: candidate.reduceMotion === true,
    compactMode: candidate.compactMode === true,
    processPanelDefaultOpen: candidate.processPanelDefaultOpen !== false,
    attachmentsEnabled: candidate.attachmentsEnabled !== false,
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

  useEffect(() => {
    document.documentElement.dataset.glassIntensity = preferences.glassIntensity;
    document.documentElement.classList.toggle("kg-reduce-motion", preferences.reduceMotion);
    document.documentElement.classList.toggle("kg-compact-ui", preferences.compactMode);

    return () => {
      delete document.documentElement.dataset.glassIntensity;
      document.documentElement.classList.remove("kg-reduce-motion", "kg-compact-ui");
    };
  }, [preferences.glassIntensity, preferences.reduceMotion, preferences.compactMode]);

  const updatePreferences = useCallback((patch: Partial<ConsolePreferences>) => {
    setPreferences((current) => normalizePreferences({ ...current, ...patch }));
  }, []);

  const resetPreferences = useCallback(() => {
    setPreferences(defaultConsolePreferences);
  }, []);

  return { preferences, updatePreferences, resetPreferences, hydrated };
}
