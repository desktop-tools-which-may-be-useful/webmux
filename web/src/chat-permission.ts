import { ChatPermissionModeSchema, type ChatPermissionMode } from "./schemas";

/** The permission modes a chat offers, in menu order. */
export const permissionChoices: { value: ChatPermissionMode; name: string }[] =
  [
    { value: "ask", name: "每次询问" },
    { value: "auto", name: "全部接受" },
  ];

/** The mode a control reported, or undefined when unknown or unchanged. */
export function changedPermissionMode(
  value: string,
  current: ChatPermissionMode,
): ChatPermissionMode | undefined {
  const parsed = ChatPermissionModeSchema.safeParse(value);
  return parsed.success && parsed.data !== current ? parsed.data : undefined;
}
