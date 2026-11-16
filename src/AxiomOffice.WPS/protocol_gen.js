// SINH TU DONG tu core-go/internal/protocol/events.json bang scripts/generate_protocol.py - KHONG sua tay.

// ES5 output: chi var va object literal.

var AxiomProtocol = AxiomProtocol || {};

AxiomProtocol.EVENTS = {
  "run.started": {
    "summary": "A run has started and its conversation/session context is ready.",
    "payload": {
      "conversationId": "string",
      "model": "string",
      "app": "string",
      "port": "int",
      "mode": "string"
    }
  },
  "run.progress": {
    "summary": "A short status line describing what the model is doing right now.",
    "payload": {
      "round": "int",
      "text": "string"
    }
  },
  "run.reasoning": {
    "summary": "Completed model reasoning text for one round (shown only when LlmShowReasoning is on).",
    "payload": {
      "round": "int",
      "text": "string",
      "model": "string"
    }
  },
  "run.reasoning.live": {
    "summary": "Streaming model reasoning tail for one round.",
    "payload": {
      "round": "int",
      "text": "string",
      "model": "string"
    }
  },
  "run.completed": {
    "summary": "The run finished successfully with a reply and usage totals.",
    "payload": {
      "reply": "string",
      "rounds": "int",
      "seconds": "number",
      "inputTokens": "int",
      "outputTokens": "int",
      "cachedTokens": "int",
      "billableTokens": "int",
      "verified": "bool",
      "model": "string",
      "undoSteps": "int",
      "editSteps": "int",
      "undoMode": "string"
    }
  },
  "run.failed": {
    "summary": "The run failed before producing a reply.",
    "payload": {
      "error": "string",
      "kind": "string",
      "undoSteps": "int",
      "editSteps": "int",
      "undoMode": "string"
    }
  },
  "run.cancelled": {
    "summary": "The run was cancelled by the user.",
    "payload": {
      "seconds": "number",
      "undoSteps": "int",
      "editSteps": "int",
      "undoMode": "string"
    }
  },
  "run.timedout": {
    "summary": "The run exceeded its time budget or the model stayed silent too long.",
    "payload": {
      "seconds": "number",
      "error": "string",
      "undoSteps": "int",
      "editSteps": "int",
      "undoMode": "string"
    }
  },
  "run.stopped": {
    "summary": "The run stopped because its token/budget limit was reached.",
    "payload": {
      "error": "string",
      "rounds": "int",
      "inputTokens": "int",
      "outputTokens": "int",
      "cachedTokens": "int",
      "billableTokens": "int",
      "undoSteps": "int",
      "editSteps": "int",
      "undoMode": "string"
    }
  },
  "run.model_switched": {
    "summary": "The run switched from the configured model to a fallback model.",
    "payload": {
      "from": "string",
      "to": "string",
      "reason": "string"
    }
  },
  "run.tracking": {
    "summary": "Track Changes was enabled for this run.",
    "payload": {
      "author": "string"
    }
  },
  "tool.started": {
    "summary": "A tool call started.",
    "payload": {
      "callId": "string",
      "tool": "string",
      "action": "string",
      "paramsPreview": "string"
    }
  },
  "tool.finished": {
    "summary": "A tool call finished, with status, timing and previews.",
    "payload": {
      "callId": "string",
      "tool": "string",
      "action": "string",
      "paramsPreview": "string",
      "ok": "bool",
      "error": "string|null",
      "ms": "int",
      "resultPreview": "string",
      "resultBytes": "int"
    }
  },
  "confirm.required": {
    "summary": "The run needs the user to confirm or choose before continuing.",
    "payload": {
      "confirmationId": "string",
      "action": "string",
      "reason": "string",
      "paramsPreview": "string|null",
      "timeoutSeconds": "int",
      "choices": "array<object>"
    }
  },
  "confirm.resolved": {
    "summary": "A pending confirmation was answered, timed out or cancelled.",
    "payload": {
      "confirmationId": "string",
      "approved": "bool",
      "by": "string",
      "choice": "string|null",
      "text": "string|null"
    }
  },
  "skill.loaded": {
    "summary": "A skill's instructions were loaded into the run context.",
    "payload": {
      "name": "string",
      "source": "string"
    }
  },
  "memory.written": {
    "summary": "A fact was added to long-term memory.",
    "payload": {
      "id": "string",
      "event": "string",
      "scope": "string",
      "text": "string"
    }
  },
  "run.todos": {
    "summary": "The run's todo list changed.",
    "payload": {
      "todos": "array<object>",
      "done": "int",
      "total": "int"
    }
  },
  "pane.quick": {
    "summary": "The pane should start a quick action requested from the ribbon or context menu.",
    "payload": {
      "quickAction": "string"
    }
  },
  "ping": {
    "summary": "Keep-alive event with no payload.",
    "payload": {}
  }
};
