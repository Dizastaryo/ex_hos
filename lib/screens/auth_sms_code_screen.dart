import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class AuthSMSCodeScreen extends StatefulWidget {
  const AuthSMSCodeScreen({super.key});

  @override
  _AuthSMSCodeScreenState createState() => _AuthSMSCodeScreenState();
}

class _AuthSMSCodeScreenState extends State<AuthSMSCodeScreen>
    with SingleTickerProviderStateMixin {
  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _otpController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  bool _isCodeSent = false;
  bool _isLoading = false;
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnackBar(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message)),
    );
  }

  Future<void> _showPasswordDialog(BuildContext context, String email) async {
    return showDialog(
      context: context,
      builder: (BuildContext context) {
        return AlertDialog(
          title: const Text("Создание пароля"),
          content: TextField(
            controller: _passwordController,
            decoration: const InputDecoration(
              labelText: 'Введите новый пароль',
              border: OutlineInputBorder(),
            ),
            obscureText: true,
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text("Отмена"),
            ),
            TextButton(
              onPressed: () async {
                String password = _passwordController.text.trim();
                if (password.isEmpty) {
                  _showSnackBar(context, "Пожалуйста, введите пароль");
                  return;
                }
                setState(() {
                  _isLoading = true;
                });
                try {
                  await Provider.of<AuthProvider>(context, listen: false)
                      .completeRegistration(email, password, "USER", "OPEN");
                  _showSnackBar(context, "Регистрация завершена успешно");
                  Navigator.pop(context);
                  Navigator.pushReplacementNamed(context, '/main');
                } catch (e) {
                  _showSnackBar(context, "Ошибка регистрации: ${e.toString()}");
                } finally {
                  setState(() {
                    _isLoading = false;
                  });
                }
              },
              child: const Text("Зарегистрироваться"),
            ),
          ],
        );
      },
    );
  }

  Widget _buildEmailField() {
    return TextField(
      controller: _emailController,
      decoration: const InputDecoration(
        labelText: 'Email',
        border: OutlineInputBorder(),
      ),
    );
  }

  Widget _buildOtpField() {
    return TextField(
      controller: _otpController,
      decoration: const InputDecoration(
        labelText: 'Введите код OTP',
        border: OutlineInputBorder(),
      ),
      keyboardType: TextInputType.number,
    );
  }

  Widget _buildRegisterTab(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          _buildEmailField(),
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: () async {
              String email = _emailController.text.trim();
              if (email.isEmpty) {
                _showSnackBar(context, "Пожалуйста, введите email");
                return;
              }
              setState(() {
                _isLoading = true;
              });
              try {
                await Provider.of<AuthProvider>(context, listen: false)
                    .signInWithEmail(email);
                setState(() {
                  _isCodeSent = true;
                });
              } catch (e) {
                _showSnackBar(context, "Ошибка отправки кода: ${e.toString()}");
              } finally {
                setState(() {
                  _isLoading = false;
                });
              }
            },
            child: const Text("Получить код"),
          ),
          if (_isCodeSent)
            Column(
              children: [
                const SizedBox(height: 20),
                _buildOtpField(),
                const SizedBox(height: 20),
                ElevatedButton(
                  onPressed: () async {
                    String otp = _otpController.text.trim();
                    if (otp.isEmpty) {
                      _showSnackBar(context, "Пожалуйста, введите OTP код");
                      return;
                    }
                    setState(() {
                      _isLoading = true;
                    });
                    try {
                      await Provider.of<AuthProvider>(context, listen: false)
                          .signInWithOtp(_emailController.text.trim(), otp);
                      _showPasswordDialog(
                          context, _emailController.text.trim());
                    } catch (e) {
                      _showSnackBar(
                          context, "Ошибка проверки кода: ${e.toString()}");
                    } finally {
                      setState(() {
                        _isLoading = false;
                      });
                    }
                  },
                  child: const Text("Подтвердить OTP"),
                ),
              ],
            ),
        ],
      ),
    );
  }

  Widget _buildLoginTab(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          _buildEmailField(),
          const SizedBox(height: 20),
          TextField(
            controller: _passwordController,
            decoration: const InputDecoration(
              labelText: 'Пароль',
              border: OutlineInputBorder(),
            ),
            obscureText: true,
          ),
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: () async {
              String email = _emailController.text.trim();
              String password = _passwordController.text.trim();
              if (email.isEmpty || password.isEmpty) {
                _showSnackBar(context, "Пожалуйста, введите email и пароль");
                return;
              }
              setState(() {
                _isLoading = true;
              });
              try {
                await Provider.of<AuthProvider>(context, listen: false)
                    .login(context, email, password);
                _showSnackBar(context, "Вход выполнен успешно");
              } catch (e) {
                _showSnackBar(context, "Ошибка при входе: ${e.toString()}");
              } finally {
                setState(() {
                  _isLoading = false;
                });
              }
            },
            child: const Text("Войти"),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Аутентификация')),
      body: Column(
        children: [
          if (_isLoading) const LinearProgressIndicator(),
          TabBar(
            controller: _tabController,
            tabs: const [
              Tab(text: 'Регистрация'),
              Tab(text: 'Вход'),
            ],
          ),
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _buildRegisterTab(context),
                _buildLoginTab(context),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
