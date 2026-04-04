import { useState, useEffect, useRef } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { Header } from "./components/Header";
import { HabitModal } from "./components/HabitModal";
import { APIClient } from "./utils/apiClient";
import { logSuccess, logError } from "./utils/logger";
import type { HabitSet, Habit, HabitWithProgress } from "./types/habit";

interface HabitsPageProps {
    onStatsClick?: () => void;
    onSettingsClick?: () => void;
}

const formatDuration = (totalSeconds: number): string => {
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
};

export const HabitsPage: FunctionalComponent<HabitsPageProps> = ({
    onStatsClick,
    onSettingsClick,
}) => {
    const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
    const [habits, setHabits] = useState<HabitWithProgress[]>([]);
    const [expandedSetId, setExpandedSetId] = useState<number | null>(null);
    const [modalState, setModalState] = useState<{
        isOpen: boolean;
        mode: "set" | "habit";
        editData?: HabitSet | Habit | null;
        setId?: number;
    }>({ isOpen: false, mode: "set" });
    const [deleteConfirm, setDeleteConfirm] = useState<{
        type: "set" | "habit";
        id: number;
        name: string;
    } | null>(null);

    const apiClientRef = useRef(new APIClient(window.location.origin));

    useEffect(() => {
        void loadData();
    }, []);

    const loadData = async () => {
        try {
            const client = apiClientRef.current;
            const sets = await client.getHabitSets();
            setHabitSets(Array.isArray(sets) ? sets : []);
            const allHabits = await client.getHabits();
            setHabits((Array.isArray(allHabits) ? allHabits : []).map((h: any) => ({
                ...h,
                today_seconds: 0,
                today_count: 0,
                progress: 0,
            })));
        } catch (e: any) {
            logError(`加载数据失败: ${e}`);
        }
    };

    const handleDelete = async () => {
        if (!deleteConfirm) return;
        try {
            const client = apiClientRef.current;
            if (deleteConfirm.type === "set") {
                await client.deleteHabitSet(deleteConfirm.id);
            } else {
                await client.deleteHabit(deleteConfirm.id);
            }
            logSuccess("✓ 删除成功");
            setDeleteConfirm(null);
            void loadData();
        } catch (e: any) {
            logError(`删除失败: ${e}`);
        }
    };

    const getHabitsBySet = (setId: number) => {
        return habits.filter((h) => h.set_id === setId);
    };

    const getCardBackgroundStyle = (habit: Habit | HabitSet) => {
        const wp = habit.wallpaper;
        if (!wp) return {};

        const isGradient = wp.startsWith("linear");
        const isImage = wp.startsWith("http");
        const isColor = wp.startsWith("#");

        if (isGradient) {
            return { background: wp };
        } else if (isImage) {
            return {
                backgroundImage: `url(${wp})`,
                backgroundSize: "cover",
                backgroundPosition: "center",
            };
        } else if (isColor) {
            return { backgroundColor: wp };
        }
        return {};
    };

    return (
        <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
            <Header
                title="习惯管理"
                showSettings={false}
                showBack={true}
                onBackClick={() => {
                    // 空回调，返回时不执行任何操作
                }}
                onStatsClick={onStatsClick}
                onSettingsClick={onSettingsClick}
            />

            <div className="flex-1 overflow-y-auto p-4">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold">习惯集</h2>
                    <button
                        className="btn btn-primary btn-sm"
                        onClick={() => setModalState({ isOpen: true, mode: "set" })}
                    >
                        + 添加
                    </button>
                </div>

                <div className="space-y-3">
                    {habitSets.map((set) => {
                        const setHabits = getHabitsBySet(set.id);
                        const isExpanded = expandedSetId === set.id;
                        return (
                            <div key={set.id} className="my-surface-card rounded-xl overflow-hidden">
                                <div
                                    className="p-4 flex items-center justify-between cursor-pointer"
                                    onClick={() => setExpandedSetId(isExpanded ? null : set.id)}
                                >
                                    <div className="flex items-center gap-3">
                                        <div
                                            className="w-4 h-4 rounded-full"
                                            style={{ backgroundColor: set.color }}
                                        />
                                        <div>
                                            <h3 className="font-semibold">{set.name}</h3>
                                            {set.description && (
                                                <p className="text-sm text-base-content/60">{set.description}</p>
                                            )}
                                            <p className="text-xs text-base-content/50">
                                                {setHabits.length} 个习惯
                                            </p>
                                        </div>
                                    </div>
                                    <div className="flex gap-2" onClick={(e: Event) => e.stopPropagation()}>
                                        <button
                                            className="btn btn-ghost btn-sm btn-circle"
                                            onClick={() => setModalState({
                                                isOpen: true,
                                                mode: "set",
                                                editData: set,
                                            })}
                                        >
                                            ✏️
                                        </button>
                                        <button
                                            className="btn btn-ghost btn-sm btn-circle"
                                            onClick={() => setDeleteConfirm({
                                                type: "set",
                                                id: set.id,
                                                name: set.name,
                                            })}
                                        >
                                            🗑️
                                        </button>
                                    </div>
                                </div>

                                {isExpanded && (
                                    <div className="border-t border-white/10 p-3 space-y-2">
                                        {setHabits.length === 0 ? (
                                            <p className="text-center text-base-content/50 py-2">
                                                暂无习惯，点击下方添加
                                            </p>
                                        ) : (
                                            setHabits.map((habit) => (
                                                <div
                                                    key={habit.id}
                                                    className="p-3 rounded-lg flex items-center justify-between cursor-pointer hover:bg-white/10 transition-colors overflow-hidden"
                                                    style={getCardBackgroundStyle(habit)}
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div
                                                            className="w-3 h-3 rounded-full shrink-0"
                                                            style={{ backgroundColor: habit.color }}
                                                        />
                                                        <div>
                                                            <p className="font-medium">{habit.name}</p>
                                                            <p className="text-xs">
                                                                目标: {formatDuration(habit.goal_seconds)}
                                                            </p>
                                                        </div>
                                                    </div>
                                                    <div className="flex gap-2 shrink-0">
                                                        <button
                                                            className="btn btn-ghost btn-sm btn-circle"
                                                            onClick={() => setModalState({
                                                                isOpen: true,
                                                                mode: "habit",
                                                                editData: habit,
                                                                setId: set.id,
                                                            })}
                                                        >
                                                            ✏️
                                                        </button>
                                                        <button
                                                            className="btn btn-ghost btn-sm btn-circle"
                                                            onClick={() => setDeleteConfirm({
                                                                type: "habit",
                                                                id: habit.id,
                                                                name: habit.name,
                                                            })}
                                                        >
                                                            🗑️
                                                        </button>
                                                    </div>
                                                </div>
                                            ))
                                        )}
                                        <button
                                            className="btn btn-ghost btn-sm w-full mt-2"
                                            onClick={() => setModalState({
                                                isOpen: true,
                                                mode: "habit",
                                                setId: set.id,
                                            })}
                                        >
                                            + 添加习惯
                                        </button>
                                    </div>
                                )}
                            </div>
                        );
                    })}
                </div>

                {habitSets.length === 0 && (
                    <div className="text-center py-12 text-base-content/50">
                        <p>暂无习惯集</p>
                        <p>点击上方"添加"创建第一个习惯集</p>
                    </div>
                )}
            </div>

            <HabitModal
                isOpen={modalState.isOpen}
                mode={modalState.mode}
                editData={modalState.editData}
                setId={modalState.setId}
                onClose={() => setModalState({ isOpen: false, mode: "set" })}
                onSuccess={() => {
                    setModalState({ isOpen: false, mode: "set" });
                    void loadData();
                }}
            />

            {deleteConfirm && (
                <div className="fixed inset-0 z-50 flex items-center justify-center">
                    <div className="absolute inset-0 bg-black/50" onClick={() => setDeleteConfirm(null)} />
                    <div className="relative bg-base-100 rounded-lg p-6 w-full max-w-sm mx-4 shadow-xl">
                        <h3 className="text-lg font-bold mb-4">确认删除</h3>
                        <p className="mb-4">
                            确定要删除"{deleteConfirm.name}"
                            {deleteConfirm.type === "set" ? "及其所有习惯" : ""}吗？
                        </p>
                        <p className="text-sm text-error mb-4">此操作不可撤销</p>
                        <div className="flex gap-2">
                            <button
                                className="btn btn-ghost flex-1"
                                onClick={() => setDeleteConfirm(null)}
                            >
                                取消
                            </button>
                            <button
                                className="btn btn-error flex-1"
                                onClick={() => void handleDelete()}
                            >
                                删除
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};
