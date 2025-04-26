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
        backgroundColor: Theme.of(context).primaryColor,
      ),
    );
  }

  Future<void> _handlePasswordReset() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    setState(() => _isLoading = true);
    final auth = Provider.of<AuthProvider>(context, listen: false);

    try {
      final login = _loginController.text.trim();

      if (!_isOtpSent) {
        await auth.requestPasswordReset(login);
        _showSnack('Код подтверждения отправлен');
        setState(() => _isOtpSent = true);
      } else {
        await auth.confirmPasswordReset(
          login,
          _otpController.text.trim(),
          _newPasswordController.text.trim(),
        );
        _showSnack('Пароль успешно изменён');
        Navigator.pop(context);
      }
    } catch (e) {
      _showSnack('Ошибка: ${e.toString().replaceAll('Exception: ', '')}');
    } finally {
      setState(() => _isLoading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Сброс пароля'),
        backgroundColor: const Color(0xFF6A0DAD),
      ),
      body: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 30),
        child: Form(
          key: _formKey,
          child: ListView(
            children: [
              const Text(
                'Введите свои данные для сброса пароля',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: Color(0xFF6A0DAD),
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 30),
              TextFormField(
                controller: _loginController,
                decoration: InputDecoration(
                  labelText: 'Email или Телефон',
                  prefixIcon: Icon(Icons.person, color: Color(0xFF6A0DAD)),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(15),
                    borderSide: BorderSide(color: Color(0xFF6A0DAD)),
                  ),
                ),
                validator: (value) {
                  if (value?.isEmpty ?? true) {
                    return 'Введите email или телефон';
                  }
                  return null;
                },
              ),
              if (_isOtpSent) ...[
                const SizedBox(height: 16),
                TextFormField(
                  controller: _otpController,
                  decoration: InputDecoration(
                    labelText: 'Код подтверждения',
                    prefixIcon:
                        Icon(Icons.lock_clock, color: Color(0xFF6A0DAD)),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(15),
                      borderSide: BorderSide(color: Color(0xFF6A0DAD)),
                    ),
                  ),
                  validator: (value) =>
                      value?.isEmpty ?? true ? 'Введите код' : null,
                ),
                const SizedBox(height: 16),
                TextFormField(
                  controller: _newPasswordController,
                  obscureText: !_isPasswordVisible,
                  decoration: InputDecoration(
                    labelText: 'Новый пароль',
                    prefixIcon:
                        const Icon(Icons.lock, color: Color(0xFF6A0DAD)),
                    suffixIcon: IconButton(
                      icon: Icon(
                        _isPasswordVisible
                            ? Icons.visibility
                            : Icons.visibility_off,
                        color: Color(0xFF6A0DAD),
                      ),
                      onPressed: () => setState(
                          () => _isPasswordVisible = !_isPasswordVisible),
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(15),
                      borderSide: BorderSide(color: Color(0xFF6A0DAD)),
                    ),
                  ),
                  validator: (value) {
                    if (value?.isEmpty ?? true) return 'Введите пароль';
                    if (value!.length < 8) return 'Минимум 8 символов';
                    if (!value.contains(RegExp(r'[A-Z]'))) {
                      return 'Добавьте заглавную букву';
                    }
                    if (!value.contains(RegExp(r'[!@#$%^&*(),.?":{}|<>]'))) {
                      return 'Добавьте спецсимвол';
                    }
                    return null;
                  },
                ),
              ],
              const SizedBox(height: 30),
              _isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : ElevatedButton(
                      onPressed: _isLoading ? null : _handlePasswordReset,
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size.fromHeight(50),
                        backgroundColor: const Color(0xFF6A0DAD),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(15),
                        ),
                      ),
                      child: Text(
                        _isOtpSent ? 'Сменить пароль' : 'Получить код',
                        style: const TextStyle(fontSize: 16),
                      ),
                    ),
            ],
          ),
        ),
      ),
    );
  }
}
