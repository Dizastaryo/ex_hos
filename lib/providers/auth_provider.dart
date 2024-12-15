import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import '../services/auth_service.dart';

class AuthProvider with ChangeNotifier {
  final AuthService _authService;
  bool _isCodeSent = false;
  bool _isLoading = false;
  dynamic currentUser;

  AuthProvider(Dio dio) : _authService = AuthService(dio);

  bool get isCodeSent => _isCodeSent;
  bool get isLoading => _isLoading;

  // Состояние загрузки
  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  // Сохранение учетных данных
  Future<void> _saveCredentials(String email, String password) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('email', email);
    await prefs.setString('password', password);
  }

  // Очистка сохраненных данных
  Future<void> _clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('email');
    await prefs.remove('password');
  }

  // Загрузка сохраненных учетных данных
  Future<Map<String, String>?> _loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    String? email = prefs.getString('email');
    String? password = prefs.getString('password');

    if (email != null && password != null) {
      return {'email': email, 'password': password};
    }
    return null;
  }

  // Отправка OTP
  Future<void> sendOtp(String email) async {
    try {
      _setLoading(true);
      await _authService.sendOtp(email);
      _isCodeSent = true;
    } catch (e) {
      throw Exception('Error sending OTP: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Подтверждение OTP
  Future<void> verifyOtp(String email, String otp) async {
    try {
      _setLoading(true);
      await _authService.verifyOtp(email, otp);
    } catch (e) {
      throw Exception('Error verifying OTP: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Завершение регистрации
  Future<void> completeRegistration(
      String email, String password, String phoneNumber) async {
    try {
      _setLoading(true);
      await _authService.completeRegistration(email, password, phoneNumber);
      await _saveCredentials(email, password);
      currentUser = {'email': email};
    } catch (e) {
      throw Exception('Error completing registration: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Вход в систему
  Future<void> login(String email, String password,
      [BuildContext? context]) async {
    try {
      _setLoading(true);

      // Проверка на админские данные для автоматического входа
      if (email == 'admin@gmail.com' && password == 'admin3721') {
        currentUser = {'email': email};
        await _saveCredentials(email, password);
        notifyListeners();

        // Переход в /main, если это администратор
        if (context != null) {
          Navigator.pushReplacementNamed(context, '/main');
        }
        return; // Выход из метода
      }

      // Выполнение запроса на сервер, если данные не для администратора
      await _authService.login(email, password);
      currentUser = {'email': email};
      await _saveCredentials(email, password);
      notifyListeners();

      if (context != null) {
        Navigator.pushReplacementNamed(context, '/main');
      }
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Выход
  Future<void> signOut() async {
    _isCodeSent = false;
    currentUser = null;
    await _clearCredentials();
    notifyListeners();
  }

  // Автоматический вход
  Future<void> autoLogin(BuildContext context) async {
    // Загружаем сохраненные учетные данные
    var credentials = await _loadCredentials();

    if (credentials != null) {
      // Если данные есть, выполняем вход с сохраненным email и паролем
      await login(credentials['email']!, credentials['password']!, context);
    } else {
      // Если данных нет, переходим на экран авторизации
      WidgetsBinding.instance.addPostFrameCallback((_) {
        Navigator.pushReplacementNamed(context, '/auth');
      });
    }
  }
}
