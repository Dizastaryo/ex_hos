import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';

class AuthService {
  // Берём базовый URL аутентификации из .env
  final String _authBaseUrl = dotenv.env['AUTH_BASE_URL']!;
  final Dio dio;

  AuthService(this.dio);

  // Отправка OTP на email
  Future<void> sendEmailOtp(String email) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/send-otp',
        data: {'email': email},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Проверка OTP для email
  Future<void> verifyEmailOtp(String email, String otp) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/verify-otp',
        data: {'email': email, 'otp': otp},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Отправка OTP на телефон
  Future<void> sendSmsOtp(String phone) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/send-sms-otp',
        data: {'phoneNumber': phone},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Проверка OTP по телефону
  Future<void> verifySmsOtp(String phone, String otp) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/verify-sms-otp',
        data: {'phoneNumber': phone, 'otp': otp},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Регистрация через email
  Future<void> registerWithEmail(
      String username, String email, String password, String otp) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/signup',
        data: {
          'username': username,
          'email': email,
          'password': password,
          'otp': otp,
          'role': ['user'],
        },
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Регистрация по телефону
  Future<void> registerWithPhone(
      String username, String phone, String password, String otp) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/signup-phone',
        data: {
          'username': username,
          'phoneNumber': phone,
          'password': password,
          'otp': otp,
          'role': ['user'],
        },
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Вход
  Future<Response> login(String login, String password) async {
    try {
      return await dio.post(
        '$_authBaseUrl/api/auth/signin',
        data: {'login': login, 'password': password},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Обновление access токена
  Future<Response> refreshToken() async {
    try {
      return await dio.post(
        '$_authBaseUrl/api/auth/refresh',
        options: Options(
          headers: {'Content-Type': 'application/json'},
          extra: {'withCredentials': true},
        ),
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Выход
  Future<void> logout() async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/logout',
        options: Options(
          headers: {'Content-Type': 'application/json'},
          extra: {'withCredentials': true},
        ),
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Запрос на сброс пароля
  Future<void> requestPasswordReset(String login) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/reset-password/request',
        data: {'login': login},
      );
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  // Подтверждение сброса пароля
  Future<void> confirmPasswordReset(
      String login, String otp, String newPassword) async {
    try {
      await dio.post(
        '$_authBaseUrl/api/auth/reset-password/confirm',
        data: {
          'login': login,
          'otp': otp,
          'newPassword': newPassword,
        },
      );
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
