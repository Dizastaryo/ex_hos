import 'dart:convert';
import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';

class ChatService {
  final String _baseUrl = dotenv.env['API_BASE_URL']!;
  final Dio _dio;

  ChatService(this._dio);

  /// Пользователь отправляет сообщение, получает диагноз
  Future<Map<String, dynamic>> sendMessage(String text) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/chat/diagnose',
        data: {'text': text},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Получить историю чата текущего пользователя
  Future<List<dynamic>> getUserChatHistory() async {
    try {
      final response = await _dio.get('$_baseUrl/chat/history');
      return response.data['messages'] as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Получить историю чата другого пользователя (для модератора)
  Future<List<dynamic>> getChatHistoryForUser(int userId) async {
    try {
      final response =
          await _dio.get('$_baseUrl/chat/moderator/history/$userId');
      return response.data['messages'] as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Отправить сообщение от модератора
  Future<void> sendModeratorReply(int userId, String message) async {
    try {
      await _dio.post(
        '$_baseUrl/chat/moderator/reply',
        data: {'user_id': userId, 'message': message},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<String> predictImage(File imageFile) async {
    try {
      final formData = FormData.fromMap({
        'file': await MultipartFile.fromFile(imageFile.path),
      });

      final response = await _dio.post(
        '$_baseUrl/predict-image',
        data: formData,
        options: Options(
          contentType: 'multipart/form-data',
        ),
      );

      return response.data['predicted_class'] as String;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }
}
