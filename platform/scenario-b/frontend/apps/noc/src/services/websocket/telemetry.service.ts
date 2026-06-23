// SPDX-License-Identifier: Apache-2.0

import type { TelemetryFrame } from "../../types";
import { generateTelemetryFrame } from "../mocks/data-generators";

type TelemetryHandlers = {
  onFrame: (frame: TelemetryFrame) => void;
  onDrop: () => void;
};

const components: Array<{ component: TelemetryFrame["component"]; componentId: string }> = [
  { component: "BESU", componentId: "besu-a-1" },
  { component: "BESU", componentId: "besu-b-1" },
  { component: "PALADIN", componentId: "paladin-a" },
  { component: "CACTI", componentId: "relay-a-hub" },
];

export const telemetryService = {
  start(handlers: TelemetryHandlers) {
    const timer = setInterval(() => {
      const selected = components[Math.floor(Math.random() * components.length)];
      handlers.onFrame(generateTelemetryFrame(selected.component, selected.componentId));

      if (Math.random() < 0.03) {
        clearInterval(timer);
        handlers.onDrop();
      }
    }, 3000);

    return () => clearInterval(timer);
  },
};
