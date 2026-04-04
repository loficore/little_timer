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
  private context: AudioContext | null = null;

  private masterGain: GainNode | null = null;

  private preferences: AudioPreferences = DEFAULT_AUDIO_PREFERENCES;

  setPreferences(prefs: AudioPreferences): void {
    this.preferences = normalizeAudioPreferences(prefs);
    if (this.masterGain) {
      this.masterGain.gain.value = this.resolveMasterGain();
    }
  }

  async unlock(): Promise<void> {
    const ready = this.ensureContext();
    if (!ready || !this.context) {
      return;
    }

    if (this.context.state === "suspended") {
      await this.context.resume();
    }
  }

  playTick(): void {
    if (!this.preferences.sound_enabled || !this.preferences.sound_tick) {
      return;
    }
    this.playPulse(1300, 0.06, 0.14, "triangle");
  }

  playFinish(): void {
    if (!this.preferences.sound_enabled || !this.preferences.sound_finish) {
      return;
    }

    this.playPulse(860, 0.12, 0.36, "sine", 0);
    this.playPulse(1080, 0.14, 0.28, "triangle", 0.16);
  }

  private resolveMasterGain(): number {
    if (!this.preferences.sound_enabled) {
      return 0;
    }
    return (this.preferences.sound_volume / 100) * 0.35;
  }

  private resolveAudioContextClass(): (new () => AudioContext) | null {
    if (typeof window === "undefined") {
      return null;
    }

    const win = window as Window & { webkitAudioContext?: new () => AudioContext };
    const nativeAudioContext =
      typeof globalThis.AudioContext === "function" ? globalThis.AudioContext : null;
    return nativeAudioContext ?? win.webkitAudioContext ?? null;
  }

  private ensureContext(): boolean {
    if (this.context && this.masterGain) {
      return true;
    }

    const AudioContextClass = this.resolveAudioContextClass();
    if (!AudioContextClass) {
      return false;
    }

    try {
      this.context = new AudioContextClass();
      this.masterGain = this.context.createGain();
      this.masterGain.gain.value = this.resolveMasterGain();
      this.masterGain.connect(this.context.destination);
      return true;
    } catch {
      this.context = null;
      this.masterGain = null;
      return false;
    }
  }

  private playPulse(
    frequency: number,
    durationSeconds: number,
    pulseGain: number,
    waveform: OscillatorType,
    delaySeconds = 0,
  ): void {
    if (!this.preferences.sound_enabled || this.preferences.sound_volume <= 0) {
      return;
    }

    const ready = this.ensureContext();
    if (!ready || !this.context || !this.masterGain) {
      return;
    }

    const startAt = this.context.currentTime + delaySeconds;
    const endAt = startAt + durationSeconds;

    const oscillator = this.context.createOscillator();
    oscillator.type = waveform;
    oscillator.frequency.value = frequency;

    const gain = this.context.createGain();
    gain.gain.setValueAtTime(0.0001, startAt);
    gain.gain.exponentialRampToValueAtTime(pulseGain, startAt + 0.01);
    gain.gain.exponentialRampToValueAtTime(0.0001, endAt);

    oscillator.connect(gain);
    gain.connect(this.masterGain);

    oscillator.start(startAt);
    oscillator.stop(endAt + 0.02);
  }
}

export const audioEngine = new AudioEngine();
