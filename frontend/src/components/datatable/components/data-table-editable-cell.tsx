"use client";

import type { Cell, RowData } from "@tanstack/react-table";
import { Pencil } from "lucide-react";
import * as React from "react";

import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { CellEditorConfig } from "../types/data-table";

interface DataTableEditableCellProps<TData extends RowData> {
    cell: Cell<TData, unknown>;
    editor: CellEditorConfig;
    displayContent: React.ReactNode;
    onCommit: (value: unknown) => void;
}

function toInputValue(value: unknown): string {
    if (value == null) return "";
    return String(value);
}

function parseDraftValue(draft: string, editorType: CellEditorConfig["type"]) {
    if (editorType === "number") {
        const parsed = Number(draft);
        return Number.isNaN(parsed) ? draft : parsed;
    }
    return draft;
}

export function DataTableEditableCell<TData extends RowData>({
    cell,
    editor,
    displayContent,
    onCommit,
}: DataTableEditableCellProps<TData>) {
    const rawValue = cell.getValue();
    const editorType = editor.type ?? "text";
    const [isEditing, setIsEditing] = React.useState(false);
    const [isActive, setIsActive] = React.useState(false);
    const [draft, setDraft] = React.useState(() => toInputValue(rawValue));
    const inputRef = React.useRef<HTMLInputElement>(null);
    const displayRef = React.useRef<HTMLDivElement>(null);

    React.useEffect(() => {
        if (!isEditing) {
            setDraft(toInputValue(rawValue));
        }
    }, [rawValue, isEditing]);

    React.useEffect(() => {
        if (isEditing && inputRef.current) {
            inputRef.current.focus();
            inputRef.current.select();
        }
    }, [isEditing]);

    const startEditing = React.useCallback(() => {
        setDraft(toInputValue(rawValue));
        setIsEditing(true);
    }, [rawValue]);

    const commit = React.useCallback(
        (nextValue: unknown) => {
            setIsEditing(false);
            if (nextValue !== rawValue) {
                onCommit(nextValue);
            }
        },
        [onCommit, rawValue],
    );

    const cancel = React.useCallback(() => {
        setDraft(toInputValue(rawValue));
        setIsEditing(false);
        displayRef.current?.focus();
    }, [rawValue]);

    const handleInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
        if (event.key === "Enter") {
            event.preventDefault();
            commit(parseDraftValue(draft, editorType));
        } else if (event.key === "Escape") {
            event.preventDefault();
            cancel();
        }
    };

    const handleDisplayKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
        if (event.key === "Enter" || event.key === "F2") {
            event.preventDefault();
            startEditing();
        }
    };

    if (editorType === "checkbox") {
        return (
            <div className="flex min-h-9 items-center justify-center px-2">
                <Checkbox
                    checked={Boolean(rawValue)}
                    onCheckedChange={(checked) => onCommit(Boolean(checked))}
                    aria-label="Toggle value"
                />
            </div>
        );
    }

    if (isEditing) {
        if (editorType === "select" && editor.options?.length) {
            return (
                <div className="min-h-9 p-0.5">
                    <Select
                        open
                        value={toInputValue(rawValue)}
                        onOpenChange={(open) => {
                            if (!open) setIsEditing(false);
                        }}
                        onValueChange={(value) => commit(value)}
                    >
                        <SelectTrigger
                            size="sm"
                            className="h-8 w-full min-w-0 border-primary/30 bg-background shadow-sm"
                        >
                            <SelectValue placeholder={editor.placeholder ?? "Select..."} />
                        </SelectTrigger>
                        <SelectContent>
                            {editor.options.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                    {option.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            );
        }

        return (
            <div className="min-h-9 p-0.5">
                <Input
                    ref={inputRef}
                    type={editorType === "number" ? "number" : "text"}
                    value={draft}
                    placeholder={editor.placeholder}
                    className="h-8 rounded-sm border-primary/30 bg-background px-2 shadow-sm focus-visible:ring-2 focus-visible:ring-primary/25"
                    onChange={(event) => setDraft(event.target.value)}
                    onBlur={() => commit(parseDraftValue(draft, editorType))}
                    onKeyDown={handleInputKeyDown}
                />
            </div>
        );
    }

    const isEmpty =
        displayContent == null ||
        (typeof displayContent === "string" && displayContent.trim() === "");

    return (
        <div
            ref={displayRef}
            role="button"
            tabIndex={0}
            title="Double-click to edit"
            aria-label="Editable cell. Double-click or press Enter to edit."
            className={cn(
                "group/cell relative flex min-h-9 w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm",
                "transition-[background-color,box-shadow] outline-none",
                isActive
                    ? "bg-background shadow-[inset_0_0_0_1px_hsl(var(--ring)/0.45)]"
                    : "hover:bg-muted/40",
                "focus-visible:bg-background focus-visible:shadow-[inset_0_0_0_1px_hsl(var(--ring)/0.45)]",
            )}
            onClick={() => setIsActive(true)}
            onDoubleClick={(event) => {
                event.preventDefault();
                startEditing();
            }}
            onBlur={() => setIsActive(false)}
            onFocus={() => setIsActive(true)}
            onKeyDown={handleDisplayKeyDown}
        >
            <span
                className={cn(
                    "min-w-0 flex-1 truncate leading-snug",
                    isEmpty && "text-muted-foreground italic",
                )}
            >
                {isEmpty ? (editor.placeholder ?? "—") : displayContent}
            </span>
            <Pencil
                className={cn(
                    "size-3 shrink-0 text-muted-foreground/70 transition-opacity",
                    isActive
                        ? "opacity-60"
                        : "opacity-0 group-hover/cell:opacity-50 group-focus-visible/cell:opacity-50",
                )}
                aria-hidden
            />
        </div>
    );
}
