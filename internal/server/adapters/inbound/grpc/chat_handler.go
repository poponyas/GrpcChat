package adapters_inbound_grpc

import (
	"errors"
	"io"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	server_service_ports "github.com/poponyas/GrpcChat/internal/server/ports"
	server_service "github.com/poponyas/GrpcChat/internal/server/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChatHandler struct {
	chat_v1.UnimplementedChatServiceServer
	chatService *server_service.ChatService
	hub         server_service_ports.ChatNotifier
}

func NewChatHandler(chatService *server_service.ChatService,
	hub server_service_ports.ChatNotifier,
) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		hub:         hub,
	}
}

func (h *ChatHandler) Connect(stream grpc.BidiStreamingServer[chat_v1.ChatMessage, chat_v1.ChatMessage]) error {
	// 1. Приводим value из контекста к типу UUID
	userID, ok := stream.Context().Value(contextJWTorIDkey).(uuid.UUID)
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid or missing user_id in context")
	}

	// 2. Создаем буферизированный канал для сообщений
	clientChan := make(chan *domain.Message, 100)

	// 3. Регистрируем в Hub и ГАРАНТИРУЕМ удаление при выходе из функции
	h.hub.RegisterClient(userID, clientChan)
	defer h.hub.UnregisterClient(userID)

	// 4. Горутина-писатель (Server -> Client)
	// ВАЖНО: Ровно ОДНА горутина, так как stream.Send не потокобезопасен
	go func() {
		for msg := range clientChan {
			pbMsg := &chat_v1.ChatMessage{
				Id:        msg.ID.String(),
				ChatId:    msg.ChatID.String(),
				SenderId:  msg.SenderID.String(),
				Content:   msg.Content,
				CreatedAt: timestamppb.New(msg.CreatedAt),
			}

			if err := stream.Send(pbMsg); err != nil {
				// Ошибка отправки означает, что клиент отключился
				return
			}
		}
	}()

	// 5. Основной цикл чтения (Client -> Server)
	// Держит метод Connect заблокированным, пока идет общение
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Клиент штатно закрыл соединение
			return nil
		}
		if err != nil {
			// Ошибка сети или соединения
			return err
		}

		chatID, err := uuid.Parse(req.GetChatId())
		if err != nil {
			continue // Или возвращаем ошибку формата
		}

		// Передаем пришедшее сообщение в наш UseCase
		_, err = h.chatService.SendMessage(
			stream.Context(),
			userID,
			chatID,
			req.GetContent(),
		)
		if err != nil {
			// Логируем ошибку сохранения/рассылки
		}
	}
}
