import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:encrypt/encrypt.dart' as encrypt;
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';

import '../services/auth_service.dart';

class AuthProvider with ChangeNotifier {
  final Dio _dio;
  final AuthService _authService;

  final PersistCookieJar _cookieJar;

  bool _isLoading = false;
  String? _accessToken;

  AuthProvider(this._dio, this._cookieJar) : _authService = AuthService(_dio);

  bool get isLoading => _isLoading;
  String? get token => _accessToken;

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  // Secure storage instance
  final FlutterSecureStorage _secureStorage = const FlutterSecureStorage();
  static const _refreshTokenKey = 'refresh_token';

  // AES encryption settings (32-byte key)
  final encrypt.Key _encryptionKey =
      encrypt.Key.fromUtf8('aidyn32lengthsupersecretnoonekno');
  final encrypt.IV _iv = encrypt.IV.fromLength(16);
  late final encrypt.Encrypter _encrypter =
      encrypt.Encrypter(encrypt.AES(_encryptionKey));

  // Save (encrypt) refresh token
  Future<void> _saveRefreshToken(String refreshToken) async {
    final encrypted = _encrypter.encrypt(refreshToken, iv: _iv);
    await _secureStorage.write(key: _refreshTokenKey, value: encrypted.base64);
  }

  // Load (decrypt) refresh token
  Future<String?> _loadRefreshToken() async {
    final encrypted = await _secureStorage.read(key: _refreshTokenKey);
    if (encrypted == null) return null;
    try {
      return _encrypter.decrypt64(encrypted, iv: _iv);
    } catch (_) {
      await _secureStorage.delete(key: _refreshTokenKey);
      return null;
    }
  }

  Future<void> _clearRefreshToken() async {
    await _secureStorage.delete(key: _refreshTokenKey);
  }

  /// Perform login: store tokens and navigate to main screen
  Future<void> login(
      String login, String password, BuildContext context) async {
    _setLoading(true);
    try {
      final resp = await _authService.login(login, password);
      if (resp.statusCode == 200) {
        final data = resp.data;
        _accessToken = data['accessToken'];
        final rt = data['refreshToken'];
        if (rt is String) await _saveRefreshToken(rt);
        notifyListeners();
        Navigator.pushReplacementNamed(context, '/main');
      }
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  /// Refresh access token
  Future<void> refreshToken() async {
    final rt = await _loadRefreshToken();
    if (rt == null) throw Exception('No refresh token found');
    try {
      final resp = await _dio.post(
        '/auth/refresh',
        options: Options(
          headers: {'Cookie': 'refreshToken=$rt'},
        ),
      );
      if (resp.statusCode == 200) {
        final data = resp.data;
        _accessToken = data['accessToken'];
        final newRt = data['refreshToken'];
        if (newRt is String) await _saveRefreshToken(newRt);
        notifyListeners();
      } else {
        throw Exception('Refresh failed: ${resp.statusCode}');
      }
    } catch (e) {
      throw Exception('Token refresh error: $e');
    }
  }

  /// Attempt auto-login: refresh token and navigate accordingly
  Future<void> autoLogin(BuildContext context) async {
    _setLoading(true);
    final rt = await _loadRefreshToken();
    if (rt != null) {
      try {
        await refreshToken();
        Navigator.pushReplacementNamed(context, '/main');
        return;
      } catch (_) {}
    }
    await _clearRefreshToken();
    _accessToken = null;
    notifyListeners();
    Navigator.pushReplacementNamed(context, '/auth');
    _setLoading(false);
  }

  Future<void> logout(BuildContext context) async {
    await _clearRefreshToken();
    _accessToken = null;
    notifyListeners();
    Navigator.pushReplacementNamed(context, '/auth');
  }

  // Proxy for password reset
  Future<void> requestPasswordReset(String login) async {
    await _authService.requestPasswordReset(login);
  }

  Future<void> confirmPasswordReset(
      String login, String otp, String newPassword) async {
    await _authService.confirmPasswordReset(login, otp, newPassword);
  }

  // Proxy other AuthService methods
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
