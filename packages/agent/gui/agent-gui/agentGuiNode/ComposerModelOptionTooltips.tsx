import { cloneElement, type HTMLAttributes, type ReactElement } from "react";
import {
  RoomsHintIcon,
  Tooltip,
  TooltipContent,
  TooltipTrigger
} from "@tutti-os/ui-system";
import type { ComposerMenuOption } from "./model/composerSettingsMenuModel";

export function ComposerOptionInfoTooltip({
  description,
  tooltipsEnabled = true
}: {
  description: string;
  tooltipsEnabled?: boolean;
}): React.JSX.Element {
  const stopSelect = (event: React.SyntheticEvent): void => {
    event.preventDefault();
    event.stopPropagation();
  };

  const trigger = (
    <span
      className="pointer-events-none inline-flex shrink-0 cursor-help text-[var(--agent-gui-text-tertiary)] opacity-0 transition-opacity group-hover/composer-option:pointer-events-auto group-hover/composer-option:opacity-100 group-data-[highlighted]/composer-option:pointer-events-auto group-data-[highlighted]/composer-option:opacity-100"
      data-agent-composer-option-info-trigger="true"
      onClick={stopSelect}
      onPointerDown={stopSelect}
    >
      <RoomsHintIcon aria-hidden className="size-3" />
    </span>
  );

  if (!tooltipsEnabled) {
    return trigger;
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>{trigger}</TooltipTrigger>
      <TooltipContent side="right" className="max-w-[240px] whitespace-normal">
        {description}
      </TooltipContent>
    </Tooltip>
  );
}

export function ComposerModelOptionTooltip({
  children,
  option,
  tooltipsEnabled = true
}: {
  children: ReactElement<HTMLAttributes<HTMLElement>>;
  option: ComposerMenuOption;
  tooltipsEnabled?: boolean;
}): React.JSX.Element {
  if (!tooltipsEnabled || !option.tooltip) {
    return children;
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {cloneElement(children, {
          "data-agent-model-option-tooltip-trigger": "true"
        } as Partial<HTMLAttributes<HTMLElement>> &
          Record<"data-agent-model-option-tooltip-trigger", string>)}
      </TooltipTrigger>
      <TooltipContent
        side="right"
        align="start"
        sideOffset={8}
        className="flex w-[320px] max-w-[calc(100vw-32px)] flex-col items-start gap-0 whitespace-normal rounded-lg border border-[var(--line-2)] bg-[var(--background-fronted)] px-4 py-3 text-[13px] leading-[1.3] text-[var(--text-primary)] shadow-lg"
        data-agent-model-option-tooltip="true"
      >
        <span className="block text-[15px] font-semibold leading-[1.2]">
          {option.tooltip.title}
        </span>
        {option.tooltip.description ? (
          <span className="mt-1.5 block text-[13px] leading-[1.35] text-[var(--text-tertiary)]">
            {option.tooltip.description}
          </span>
        ) : null}
        {option.tooltip.contextWindow ? (
          <span className="mt-4 block">{option.tooltip.contextWindow}</span>
        ) : null}
        {option.tooltip.version ? (
          <span className="mt-4 block italic">{option.tooltip.version}</span>
        ) : null}
      </TooltipContent>
    </Tooltip>
  );
}
