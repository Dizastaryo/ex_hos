import 'package:dio/dio.dart';

class AuthService {
  final Dio dio;

  AuthService(this.dio);

  // Отправка OTP на email
  Future<void> sendEmailOtp(String email) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/send-otp', // Используем HTTPS
      data: {'email': email},
    );
  }

  // Проверка OTP для email
  Future<void> verifyEmailOtp(String email, String otp) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/verify-otp', // Используем HTTPS
      data: {'email': email, 'otp': otp},
    );
  }

  // Отправка OTP на телефон
  Future<void> sendSmsOtp(String phone) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/send-sms-otp', // Используем HTTPS
      data: {'phoneNumber': phone},
    );
  }

  // Проверка OTP по телефону
  Future<void> verifySmsOtp(String phone, String otp) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/verify-sms-otp', // Используем HTTPS
      data: {'phoneNumber': phone, 'otp': otp},
    );
  }

  // Регистрация через email
  Future<void> registerWithEmail(
      String username, String email, String password, String otp) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/signup', // Используем HTTPS
      data: {
        'username': username,
        'email': email,
        'password': password,
        'otp': otp,
        'role': ['user'],
      },
    );
  }

  // Регистрация по телефону
  Future<void> registerWithPhone(
      String username, String phone, String password, String otp) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/signup-phone', // Используем HTTPS
      data: {
        'username': username,
        'phoneNumber': phone,
        'password': password,
        'otp': otp,
        'role': ['user'],
      },
    );
  }

  // Вход
  Future<Response> login(String login, String password) async {
    return await dio.post(
      'https://172.20.10.2:8443/api/auth/signin', // Используем HTTPS
      data: {'login': login, 'password': password},
    );
  }

  // Обновление access токена
  Future<Response> refreshToken() async {
    return await dio.post(
      'https://172.20.10.2:8443/api/auth/refresh', // Используем HTTPS
      options: Options(
        headers: {'Content-Type': 'application/json'},
        extra: {'withCredentials': true}, // важно для отправки куки
      ),
    );
  }

  // Выход
  Future<void> logout() async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/logout', // Используем HTTPS
      options: Options(
        headers: {'Content-Type': 'application/json'},
        extra: {'withCredentials': true},
      ),
    );
  }

  // Запрос на сброс пароля
  Future<void> requestPasswordReset(String login) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/reset-password/request', // Используем HTTPS
      data: {'login': login},
    );
  }

  // Подтверждение сброса пароля
  Future<void> confirmPasswordReset(
      String login, String otp, String newPassword) async {
    await dio.post(
      'https://172.20.10.2:8443/api/auth/reset-password/confirm', // Используем HTTPS
      data: {
        'login': login,
        'otp': otp,
        'newPassword': newPassword,
      },
    );
  }
}
