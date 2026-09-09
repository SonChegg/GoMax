package dispatch

import (
	"encoding/json"

	"github.com/SonChegg/PyMax/protocol"
	"github.com/SonChegg/PyMax/types"
)

// Resolve maps an inbound frame's opcode/payload to a dispatchable
// EventType, a port of pymax's dispatch.resolvers + dispatch.mapping.EventResolver.
func Resolve(frame protocol.InboundFrame) (EventType, bool) {
	if frame.Cmd != protocol.CommandRequest {
		return "", false
	}

	switch frame.Opcode {
	case protocol.OpcodeNotifMessage, protocol.OpcodeMsgEdit:
		return resolveMessage(frame)
	case protocol.OpcodeNotifChat:
		return EventChatUpdate, true
	case protocol.OpcodeNotifMsgDelete:
		return EventMessageDelete, true
	case protocol.OpcodeNotifAttach:
		return resolveAttach(frame)
	case protocol.OpcodeNotifTyping:
		return EventTyping, true
	case protocol.OpcodeNotifMark:
		return EventMessageRead, true
	case protocol.OpcodeNotifPresence:
		return EventPresence, true
	case protocol.OpcodeNotifMsgReactionsChanged:
		return EventReactionUpdate, true
	default:
		return "", false
	}
}

func resolveMessage(frame protocol.InboundFrame) (EventType, bool) {
	var msg types.Message
	if !remarshal(frame.Payload, &msg) {
		return "", false
	}

	switch {
	case msg.Status != nil && *msg.Status == types.MessageStatusEdited:
		return EventMessageEdit, true
	case msg.Status != nil && *msg.Status == types.MessageStatusRemoved:
		return EventMessageDelete, true
	default:
		return EventMessageNew, true
	}
}

// resolveAttach distinguishes the three "attachment ready" notifications
// pymax tries to parse in turn (file/video/voice), a port of pymax's
// dispatch.resolvers.resolve_attach.
func resolveAttach(frame protocol.InboundFrame) (EventType, bool) {
	if _, ok := frame.Payload["fileId"]; ok {
		return EventFileReady, true
	}
	if _, ok := frame.Payload["videoId"]; ok {
		return EventVideoReady, true
	}
	if _, ok := frame.Payload["audioId"]; ok {
		return EventVoiceReady, true
	}
	return "", false
}

// Map decodes an inbound frame's payload into the concrete Go type matching
// eventType, a port of pymax's dispatch.mapping.EventMapper.map.
func Map(eventType EventType, frame protocol.InboundFrame) any {
	if frame.Cmd != protocol.CommandRequest || frame.Payload == nil {
		return frame
	}

	switch eventType {
	case EventMessageNew, EventMessageEdit:
		var v types.Message
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventChatUpdate:
		if chatRaw, ok := frame.Payload["chat"]; ok {
			var v types.Chat
			if remarshal(chatRaw, &v) {
				return &v
			}
		}
	case EventMessageDelete:
		var v types.MessageDeleteEvent
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventMessageRead:
		var v types.MessageReadEvent
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventTyping:
		var v types.TypingEvent
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventPresence:
		var v types.PresenceEvent
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventReactionUpdate:
		var v types.ReactionUpdateEvent
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventVideoReady:
		var v types.VideoUploadSignal
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventFileReady:
		var v types.FileUploadSignal
		if remarshal(frame.Payload, &v) {
			return &v
		}
	case EventVoiceReady:
		var v types.AudioUploadSignal
		if remarshal(frame.Payload, &v) {
			return &v
		}
	}

	return frame
}

// remarshal round-trips v (a map[string]any, or any JSON-marshalable value)
// through encoding/json into dst, standing in for pydantic's
// model_validate() on an already-decoded payload dict.
func remarshal(v any, dst any) bool {
	data, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dst) == nil
}
