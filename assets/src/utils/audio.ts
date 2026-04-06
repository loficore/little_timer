import tickSoundUrl from "../../audio/clock.mp3";
import finishSoundUrl from "../../audio/ding.mp3";

export interface AudioPreferences {
  sound_enabled: boolean;
  sound_tick: boolean;
  sound_finish: boolean;
  sound_volume: number;
}

export const DEFAULT_AUDIO_PREFERENCES: AudioPreferences = {
  sound_enabled: true,
  sound_tick: false,
  sound_finish: true,
  sound_volume: 35,
};

const AUDIO_STORAGE_KEY = "little_timer_audio_preferences";

const clampVolume = (value: number): number => Math.min(100, Math.max(0, Math.round(value)));

export const normalizeAudioPreferences = (
  value?: Partial<AudioPreferences> | null,
): AudioPreferences => ({
  sound_enabled: value?.sound_enabled ?? DEFAULT_AUDIO_PREFERENCES.sound_enabled,
  sound_tick: value?.sound_tick ?? DEFAULT_AUDIO_PREFERENCES.sound_tick,
  sound_finish: value?.sound_finish ?? DEFAULT_AUDIO_PREFERENCES.sound_finish,
  sound_volume: clampVolume(value?.sound_volume ?? DEFAULT_AUDIO_PREFERENCES.sound_volume),
});

export const loadAudioPreferences = (): AudioPreferences => {
  if (typeof window === "undefined") {
    return DEFAULT_AUDIO_PREFERENCES;
  }

  try {
    const raw = window.localStorage.getItem(AUDIO_STORAGE_KEY);
    if (!raw) {
      return DEFAULT_AUDIO_PREFERENCES;
    }

    return normalizeAudioPreferences(JSON.parse(raw) as Partial<AudioPreferences>);
  } catch {
    return DEFAULT_AUDIO_PREFERENCES;
  }
};

export const saveAudioPreferences = (prefs: AudioPreferences): void => {
  if (typeof window === "undefined") {
    return;
  }

  try {
    window.localStorage.setItem(AUDIO_STORAGE_KEY, JSON.stringify(normalizeAudioPreferences(prefs)));
  } catch {
    // 本地存储失败时不阻断主流程
  }
};

class AudioEngine {
  private tickAudio: HTMLAudioElement | null = null;

  private finishAudio: HTMLAudioElement | null = null;

  private preferences: AudioPreferences = DEFAULT_AUDIO_PREFERENCES;

  setPreferences(prefs: AudioPreferences): void {
    this.preferences = normalizeAudioPreferences(prefs);
    if (!this.preferences.sound_enabled || !this.preferences.sound_tick) {
      this.stopTick();
    }
  }

  async unlock(): Promise<void> {
    this.ensureAudioElements();
    return Promise.resolve();
  }

  playTick(): void {
    if (!this.preferences.sound_enabled || !this.preferences.sound_tick) {
      return;
    }

    this.playAudio(this.getTickAudio(), true);
  }

  stopTick(): void {
    const audio = this.tickAudio;
    if (!audio) {
      return;
    }

    try {
      audio.pause();
      audio.currentTime = 0;
    } catch {
      // 忽略停止过程中的浏览器限制
    }
  }

  playFinish(): void {
    this.stopTick();
    if (!this.preferences.sound_enabled || !this.preferences.sound_finish) {
      return;
    }

    this.playAudio(this.getFinishAudio());
  }

  private ensureAudioElements(): void {
    if (!this.tickAudio) {
      this.tickAudio = new Audio(tickSoundUrl);
      this.tickAudio.loop = true;
      this.tickAudio.preload = "auto";
    }

    if (!this.finishAudio) {
      this.finishAudio = new Audio(finishSoundUrl);
      this.finishAudio.preload = "auto";
    }
  }

  private getTickAudio(): HTMLAudioElement {
    this.ensureAudioElements();
    return this.tickAudio as HTMLAudioElement;
  }

  private getFinishAudio(): HTMLAudioElement {
    this.ensureAudioElements();
    return this.finishAudio as HTMLAudioElement;
  }

  private playAudio(audio: HTMLAudioElement, loop = false): void {
    if (this.preferences.sound_volume <= 0) {
      return;
    }

    try {
      audio.currentTime = 0;
      audio.loop = loop;
      audio.volume = this.preferences.sound_volume / 100;
      void audio.play().catch(() => {
        // 浏览器限制或解码失败时静默降级
      });
    } catch {
      // 浏览器不支持音频播放时保持静默
    }
  }
}

export const audioEngine = new AudioEngine();
