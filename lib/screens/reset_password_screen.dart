import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class ResetPasswordScreen extends StatefulWidget {
  const ResetPasswordScreen({super.key});

  @override
  State<ResetPasswordScreen> createState() => _ResetPasswordScreenState();
}

class _ResetPasswordScreenState extends State<ResetPasswordScreen> {
  final _formKey = GlobalKey<FormState>();
  final TextEditingController _loginController = TextEditingController();
  final TextEditingController _otpController = TextEditingController();
  final TextEditingController _newPasswordController = TextEditingController();
  bool _isOtpSent = false;
  bool _isPasswordVisible = false;
  bool _isLoading = false;

  void _showSnack(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
          content: Text(message),
          backgroundColor: Theme.of(context).primaryColor),
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = Provider.of<AuthProvider>(context, listen: false);

    return Scaffold(
      appBar: AppBar(title: const Text('Сброс пароля')),
      body: Padding(
        padding: const EdgeInsets.all(20),
        child: Form(
          key: _formKey,
          child: ListView(
            children: [
              const SizedBox(height: 20),
              TextFormField(
                controller: _loginController,
                decoration: const InputDecoration(
                  labelText: 'Email или Телефон',
                  prefixIcon: Icon(Icons.person),
                ),
                validator: (value) {
                  if (value == null || value.isEmpty)
                    return 'Введите email или телефон';
                  return null;
                },
              ),
              if (_isOtpSent) ...[
                const SizedBox(height: 16),
                TextFormField(
                  controller: _otpController,
                  decoration: const InputDecoration(
                    labelText: 'OTP',
                    prefixIcon: Icon(Icons.lock_clock),
                  ),
                  validator: (value) =>
                      value == null || value.isEmpty ? 'Введите OTP' : null,
                ),
                const SizedBox(height: 16),
                TextFormField(
                  controller: _newPasswordController,
                  obscureText: !_isPasswordVisible,
                  decoration: InputDecoration(
                    labelText: 'Новый пароль',
                    prefixIcon: const Icon(Icons.lock),
                    suffixIcon: IconButton(
                      icon: Icon(
                        _isPasswordVisible
                            ? Icons.visibility
                            : Icons.visibility_off,
                      ),
                      onPressed: () {
                        setState(
                            () => _isPasswordVisible = !_isPasswordVisible);
                      },
                    ),
                  ),
                  validator: (value) {
                    if (value == null || value.isEmpty)
                      return 'Введите новый пароль';
                    if (value.length < 6) return 'Минимум 6 символов';
                    return null;
                  },
                ),
              ],
              const SizedBox(height: 30),
              _isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : ElevatedButton(
                      onPressed: () async {
                        if (_formKey.currentState?.validate() ?? false) {
                          setState(() => _isLoading = true);
                          try {
                            final login = _loginController.text.trim();
                            if (!_isOtpSent) {
                              if (login.contains('@')) {
                                await auth.sendEmailOtp(login);
                                _showSnack('OTP отправлен на email');
                              } else {
                                await auth.sendSmsOtp(login);
                                _showSnack('OTP отправлен на телефон');
                              }
                              setState(() => _isOtpSent = true);
                            } else {
                              final otp = _otpController.text.trim();
                              final password =
                                  _newPasswordController.text.trim();
                              await auth.confirmPasswordReset(
                                  login, otp, password);
                              _showSnack('Пароль успешно сброшен');
                              Navigator.pop(context);
                            }
                          } catch (e) {
                            _showSnack('Ошибка: $e');
                          } finally {
                            setState(() => _isLoading = false);
                          }
                        }
                      },
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size.fromHeight(50),
                        backgroundColor: Theme.of(context).primaryColor,
                      ),
                      child: Text(
                          _isOtpSent ? 'Сбросить пароль' : 'Отправить OTP'),
                    ),
            ],
          ),
        ),
      ),
    );
  }
}
