import 'package:dio/dio.dart';

class AuthService {
  final Dio dio;

  AuthService(this.dio);

  // Отправка OTP на email
  Future<void> sendEmailOtp(String email) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/send-otp',
      data: {'email': email},
    );
  }

  // Проверка OTP для email
  Future<void> verifyEmailOtp(String email, String otp) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/verify-otp',
      data: {'email': email, 'otp': otp},
    );
  }

  // Отправка OTP на телефон
  Future<void> sendSmsOtp(String phone) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/send-sms-otp',
      data: {'phoneNumber': phone},
    );
  }

  // Проверка OTP по телефону
  Future<void> verifySmsOtp(String phone, String otp) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/verify-sms-otp',
      data: {'phoneNumber': phone, 'otp': otp},
    );
  }

  // Регистрация через email
  Future<void> registerWithEmail(
      String username, String email, String password, String otp) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/signup',
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
      'http://172.20.10.2:8081/api/auth/signup-phone',
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
      'http://172.20.10.2:8081/api/auth/signin',
      data: {'login': login, 'password': password},
    );
  }

  // Обновление access токена
  Future<Response> refreshToken() async {
    return await dio.post(
      'http://172.20.10.2:8081/api/auth/refresh',
      options: Options(
        headers: {'Content-Type': 'application/json'},
        extra: {'withCredentials': true}, // важно для отправки куки
      ),
    );
  }

  // Выход
  Future<void> logout() async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/logout',
      options: Options(
        headers: {'Content-Type': 'application/json'},
        extra: {'withCredentials': true},
      ),
    );
  }

  // Запрос на сброс пароля
  Future<void> requestPasswordReset(String login) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/reset-password/request',
      data: {'login': login},
    );
  }

  // Подтверждение сброса пароля
  Future<void> confirmPasswordReset(
      String login, String otp, String newPassword) async {
    await dio.post(
      'http://172.20.10.2:8081/api/auth/reset-password/confirm',
      data: {
        'login': login,
        'otp': otp,
        'newPassword': newPassword,
      },
    );
  }
}
