import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import '../services/auth_service.dart';
import 'dart:io';

class AuthProvider with ChangeNotifier {
  final Dio _dio;
  final CookieJar _cookieJar;
  final AuthService _authService;
  bool _isLoading = false;
  dynamic currentUser;
  String? _token;
  final Uri _baseUri = Uri.parse('https://172.20.10.2:8443');

  AuthProvider(this._dio, this._cookieJar) : _authService = AuthService(_dio) {
    _dio.interceptors.add(CookieManager(_cookieJar));
  }

  bool get isLoading => _isLoading;
  String? get token => _token;

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  Future<void> _saveRefreshToken() async {
    // достаём куку refreshToken из jar
    final cookies = await _cookieJar.loadForRequest(_baseUri);
    final cookie = cookies.firstWhere(
      (c) => c.name == 'refreshToken',
      orElse: () => Cookie('', ''),
    );
    if (cookie.value.isNotEmpty) {
      final prefs = await SharedPreferences.getInstance();
      // Сохраняем токен без шифрования
      await prefs.setString('refreshToken', cookie.value);
    }
  }

  Future<String?> _loadRefreshToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString('refreshToken');
  }

  Future<void> _clearRefreshToken() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('refreshToken');
    await _cookieJar.deleteAll();
  }

  Future<void> login(String login, String password,
      [BuildContext? context]) async {
    try {
      _setLoading(true);
      final response = await _authService.login(login, password);
      if (response.statusCode == 200) {
        _token = response.data['accessToken'];
        currentUser = {
          'username': response.data['username'],
          'email': response.data['email'],
          'roles': response.data['roles'],
        };

        await _saveRefreshToken();
        notifyListeners();

        if (context != null) {
          final roles = List<String>.from(response.data['roles']);
          _navigateBasedOnRole(context, roles);
        }
      }
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  Future<void> silentAutoLogin() async {
    final stored = await _loadRefreshToken();
    if (stored != null) {
      try {
        _setLoading(true);

        await _cookieJar.saveFromResponse(
          _baseUri,
          [Cookie('refreshToken', stored)],
        );

        final response = await _authService.refreshToken();
        if (response.statusCode == 200) {
          _token = response.data['accessToken'];
          currentUser = {
            'username': response.data['username'],
            'email': response.data['email'],
            'roles': response.data['roles'],
          };

          await _saveRefreshToken();
          notifyListeners();
        }
      } catch (e) {
        debugPrint('silentAutoLogin error: $e');
        rethrow;
      } finally {
        _setLoading(false);
      }
    } else {
      throw Exception('No stored refreshToken');
    }
  }

  Future<void> autoLogin(BuildContext context) async {
    final stored = await _loadRefreshToken();
    if (stored != null) {
      try {
        _setLoading(true);

        await _cookieJar.saveFromResponse(
          _baseUri,
          [Cookie('refreshToken', stored)],
        );

        final response = await _authService.refreshToken();
        if (response.statusCode == 200) {
          _token = response.data['accessToken'];
          currentUser = {
            'username': response.data['username'],
            'email': response.data['email'],
            'roles': response.data['roles'],
          };

          await _saveRefreshToken();
          notifyListeners();
          final roles = List<String>.from(response.data['roles']);
          _navigateBasedOnRole(context, roles);
          return;
        }
      } catch (e) {
        debugPrint('autoLogin refresh error: $e');
      } finally {
        _setLoading(false);
      }
    }

    // если не авторизовались — на экран аутентификации
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Navigator.pushReplacementNamed(context, '/auth');
    });
  }

  Future<void> logout(BuildContext context) async {
    await _clearRefreshToken();
    _token = null;
    currentUser = null;
    notifyListeners();
    Navigator.pushReplacementNamed(context, '/auth');
  }

  void _navigateBasedOnRole(BuildContext context, List<String> roles) {
    final route = roles.contains('ROLE_ADMIN')
        ? '/admin-home'
        : roles.contains('ROLE_MODERATOR')
            ? '/moderator-home'
            : '/main';
    Navigator.pushReplacementNamed(context, route);
  }

  Future<void> refreshToken() async {
    // Этот метод теперь используется только внутри autoLogin()
    try {
      _setLoading(true);
      final response = await _authService.refreshToken();
      if (response.statusCode == 200) {
        _token = response.data['accessToken'];
        currentUser = {
          'username': response.data['username'],
          'email': response.data['email'],
          'roles': response.data['roles'],
        };
        await _saveRefreshToken();
        notifyListeners();
      } else {
        throw Exception('Не удалось обновить токен: ${response.statusCode}');
      }
    } catch (e) {
      debugPrint('refreshToken error: $e');
      rethrow;
    } finally {
      _setLoading(false);
    }
  }

  Future<void> requestPasswordReset(String login) =>
      _authService.requestPasswordReset(login);

  Future<void> confirmPasswordReset(
          String login, String otp, String newPassword) =>
      _authService.confirmPasswordReset(login, otp, newPassword);

  Future<void> sendEmailOtp(String email) => _authService.sendEmailOtp(email);
  Future<void> verifyEmailOtp(String email, String otp) =>
      _authService.verifyEmailOtp(email, otp);
  Future<void> sendSmsOtp(String phone) => _authService.sendSmsOtp(phone);
  Future<void> verifySmsOtp(String phone, String otp) =>
      _authService.verifySmsOtp(phone, otp);
  Future<void> registerWithEmail(
          String username, String email, String password, String otp) =>
      _authService.registerWithEmail(username, email, password, otp);
  Future<void> registerWithPhone(
          String username, String phone, String password, String otp) =>
      _authService.registerWithPhone(username, phone, password, otp);
}
