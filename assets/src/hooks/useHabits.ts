/**
 * 习惯数据管理 Hook
 * 统一管理习惯集、习惯和会话数据
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { HabitSet, Habit, HabitWithProgress, HabitDetail } from "../types/habit";

export interface UseHabitsReturn {
  // 状态
  habitSets: HabitSet[];
  habits: HabitWithProgress[];
  isLoading: boolean;
  error: string | null;
  
  // 操作
  refresh: () => Promise<void>;
  createSet: (name: string, description: string, color: string) => Promise<HabitSet | null>;
  updateSet: (id: number, name: string, description: string, color: string) => Promise<void>;
  deleteSet: (id: number) => Promise<void>;
  createHabit: (setId: number, name: string, goalSeconds: number, color: string) => Promise<Habit | null>;
  updateHabit: (id: number, name: string, goalSeconds: number, color: string) => Promise<void>;
  deleteHabit: (id: number) => Promise<void>;
  getHabitsBySet: (setId: number) => HabitWithProgress[];
  getHabitDetail: (habitId: number) => Promise<HabitDetail | null>;
}

export const useHabits = (): UseHabitsReturn => {
  const apiClientRef = useRef(getAPIClient());
  const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
  const [habits, setHabits] = useState<HabitWithProgress[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const client = apiClientRef.current;
      const [setsData, habitsData] = await Promise.all([
        client.getHabitSets(),
        client.getHabits(),
      ]);
      setHabitSets(Array.isArray(setsData) ? setsData : []);
      setHabits(
        (Array.isArray(habitsData) ? habitsData : []).map((h: Habit) => ({
          ...h,
          today_seconds: 0,
          today_count: 0,
          progress: 0,
        }))
      );
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      logError(`加载习惯数据失败: ${msg}`);
      setError(msg);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const createSet = useCallback(
    async (name: string, description: string, color: string): Promise<HabitSet | null> => {
      try {
        const client = apiClientRef.current;
        const newSet = await client.createHabitSet(name, description, color);
        await refresh();
        return newSet;
      } catch (e) {
        logError(`创建习惯集失败: ${e}`);
        return null;
      }
    },
    [refresh]
  );

  const updateSet = useCallback(
    async (id: number, name: string, description: string, color: string): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.updateHabitSet(id, name, description, color);
        await refresh();
      } catch (e) {
        logError(`更新习惯集失败: ${e}`);
      }
    },
    [refresh]
  );

  const deleteSet = useCallback(
    async (id: number): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.deleteHabitSet(id);
        await refresh();
      } catch (e) {
        logError(`删除习惯集失败: ${e}`);
      }
    },
    [refresh]
  );

  const createHabit = useCallback(
    async (
      setId: number,
      name: string,
      goalSeconds: number,
      color: string
    ): Promise<Habit | null> => {
      try {
        const client = apiClientRef.current;
        const newHabit = await client.createHabit(setId, name, goalSeconds, color);
        await refresh();
        return newHabit;
      } catch (e) {
        logError(`创建习惯失败: ${e}`);
        return null;
      }
    },
    [refresh]
  );

  const updateHabit = useCallback(
    async (
      id: number,
      name: string,
      goalSeconds: number,
      color: string
    ): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.updateHabit(id, name, goalSeconds, color);
        await refresh();
      } catch (e) {
        logError(`更新习惯失败: ${e}`);
      }
    },
    [refresh]
  );

  const deleteHabit = useCallback(
    async (id: number): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.deleteHabit(id);
        await refresh();
      } catch (e) {
        logError(`删除习惯失败: ${e}`);
      }
    },
    [refresh]
  );

  const getHabitsBySet = useCallback(
    (setId: number): HabitWithProgress[] => {
      return habits.filter((h) => h.set_id === setId);
    },
    [habits]
  );

  const getHabitDetail = useCallback(
    async (habitId: number): Promise<HabitDetail | null> => {
      try {
        const client = apiClientRef.current;
        const today = new Date().toISOString().split("T")[0];
        return await client.getHabitDetail(habitId, today);
      } catch (e) {
        logError(`获取习惯详情失败: ${e}`);
        return null;
      }
    },
    []
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return {
    habitSets,
    habits,
    isLoading,
    error,
    refresh,
    createSet,
    updateSet,
    deleteSet,
    createHabit,
    updateHabit,
    deleteHabit,
    getHabitsBySet,
    getHabitDetail,
  };
};
