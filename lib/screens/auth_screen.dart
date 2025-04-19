import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:lottie/lottie.dart';
import '../providers/auth_provider.dart';
import 'package:flutter_animate/flutter_animate.dart';

enum RegistrationMode { email, phone }

class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key});

  @override
  _AuthScreenState createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  final _regFormKey = GlobalKey<FormState>();
  final TextEditingController _regUsernameController = TextEditingController();
  final TextEditingController _regEmailController = TextEditingController();
  final TextEditingController _regPhoneController = TextEditingController();
  final TextEditingController _regOtpController = TextEditingController();
  final TextEditingController _regPasswordController = TextEditingController();
  RegistrationMode _regMode = RegistrationMode.email;
  bool _isRegCodeSent = false;

  final _loginFormKey = GlobalKey<FormState>();
  final TextEditingController _loginController = TextEditingController();
  final TextEditingController _loginPasswordController =
      TextEditingController();
  bool _isPasswordVisible = false;

  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
          content: Text(msg), backgroundColor: Theme.of(context).primaryColor),
    );
  }

  Widget _buildToggle() {
    return ToggleButtons(
      isSelected: [
        _regMode == RegistrationMode.email,
        _regMode == RegistrationMode.phone,
      ],
      onPressed: (index) {
        setState(() {
          _regMode = RegistrationMode.values[index];
          _isRegCodeSent = false;
        });
      },
      borderRadius: BorderRadius.circular(8),
      selectedColor: Colors.white,
      fillColor: Theme.of(context).primaryColor,
      children: const [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 16),
          child: Text('Email'),
        ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 16),
          child: Text('Phone'),
        )
      ],
    );
  }

  Widget _buildTextField({
    required String hint,
    required IconData icon,
    required TextEditingController controller,
    required String? Function(String?) validator,
    bool obscure = false,
    VoidCallback? toggleVisibility,
  }) {
    return TextFormField(
      controller: controller,
      obscureText: obscure,
      validator: validator,
      style: TextStyle(color: Theme.of(context).primaryColor),
      decoration: InputDecoration(
        hintText: hint,
        prefixIcon: Icon(icon, color: Theme.of(context).primaryColor),
        suffixIcon: toggleVisibility != null
            ? IconButton(
                icon: Icon(obscure ? Icons.visibility_off : Icons.visibility,
                    color: Theme.of(context).primaryColor),
                onPressed: toggleVisibility,
              )
            : null,
        enabledBorder: OutlineInputBorder(
          borderSide: BorderSide(color: Theme.of(context).primaryColor),
        ),
        focusedBorder: OutlineInputBorder(
          borderSide: BorderSide(color: Theme.of(context).primaryColor),
        ),
        filled: true,
        fillColor: Colors.white,
      ),
    );
  }

  Widget _buildRegisterTab() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _regFormKey,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _buildToggle(),
            const SizedBox(height: 16),
            if (!_isRegCodeSent) ...[
              // Показывать поле username только до отправки OTP
              _buildTextField(
                hint: 'Username',
                icon: Icons.person,
                controller: _regUsernameController,
                validator: (v) =>
                    v != null && v.isEmpty ? 'Введите имя пользователя' : null,
              ),
              const SizedBox(height: 16),
            ],
            if (!_isRegCodeSent && _regMode == RegistrationMode.email)
              _buildTextField(
                hint: 'Email',
                icon: Icons.email,
                controller: _regEmailController,
                validator: (v) {
                  if (v == null || v.isEmpty) return 'Введите email';
                  if (!RegExp(r'^[^@]+@[^@]+\.[^@]+').hasMatch(v))
                    return 'Некорректный email';
                  return null;
                },
              ),
            if (!_isRegCodeSent && _regMode == RegistrationMode.phone)
              _buildTextField(
                hint: 'Phone (+77000000000)',
                icon: Icons.phone,
                controller: _regPhoneController,
                validator: (v) =>
                    v != null && v.isEmpty ? 'Введите телефон' : null,
              ),
            const SizedBox(height: 16),
            if (!_isRegCodeSent)
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: Theme.of(context).primaryColor,
                  minimumSize: const Size.fromHeight(50),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                ),
                onPressed: () async {
                  if (_regFormKey.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      if (_regMode == RegistrationMode.email) {
                        await auth
                            .sendEmailOtp(_regEmailController.text.trim());
                        _showSnack('OTP sent to email');
                      } else {
                        await auth.sendSmsOtp(_regPhoneController.text.trim());
                        _showSnack('OTP sent to phone');
                      }
                      setState(() => _isRegCodeSent = true);
                    } catch (e) {
                      _showSnack('Ошибка: $e');
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
                child: Text('Send OTP'.toUpperCase()),
              ),
            if (_isRegCodeSent) ...[
              const SizedBox(height: 16),
              _buildTextField(
                hint: 'Enter OTP',
                icon: Icons.sms,
                controller: _regOtpController,
                validator: (v) => v != null && v.isEmpty ? 'Введите OTP' : null,
              ),
              const SizedBox(height: 16),
              _buildTextField(
                hint: 'Password',
                icon: Icons.lock,
                controller: _regPasswordController,
                obscure: !_isPasswordVisible,
                toggleVisibility: () =>
                    setState(() => _isPasswordVisible = !_isPasswordVisible),
                validator: (v) {
                  if (v == null || v.isEmpty) return 'Введите пароль';
                  if (v.length < 6) return 'Минимум 6 символов';
                  return null;
                },
              ),
              const SizedBox(height: 20),
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: Theme.of(context).primaryColor,
                  minimumSize: const Size.fromHeight(50),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                ),
                onPressed: () async {
                  if (_regFormKey.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      if (_regMode == RegistrationMode.email) {
                        await auth.verifyEmailOtp(
                          _regEmailController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                        await auth.registerWithEmail(
                          _regUsernameController.text.trim(),
                          _regEmailController.text.trim(),
                          _regPasswordController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                      } else {
                        await auth.verifySmsOtp(
                          _regPhoneController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                        await auth.registerWithPhone(
                          _regUsernameController.text.trim(),
                          _regPhoneController.text.trim(),
                          _regPasswordController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                      }
                      Navigator.pushReplacementNamed(context, '/main');
                    } catch (e) {
                      _showSnack('Ошибка: $e');
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
                child: Text('Complete Registration'.toUpperCase()),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildLoginTab() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _loginFormKey,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _buildTextField(
              hint: 'Email/Username/Phone',
              icon: Icons.person,
              controller: _loginController,
              validator: (v) => v != null && v.isEmpty ? 'Введите логин' : null,
            ),
            const SizedBox(height: 16),
            _buildTextField(
              hint: 'Password',
              icon: Icons.lock,
              controller: _loginPasswordController,
              obscure: !_isPasswordVisible,
              toggleVisibility: () =>
                  setState(() => _isPasswordVisible = !_isPasswordVisible),
              validator: (v) {
                if (v == null || v.isEmpty) return 'Введите пароль';
                if (v.length < 6) return 'Минимум 6 символов';
                return null;
              },
            ),
            const SizedBox(height: 20),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: Theme.of(context).primaryColor,
                minimumSize: const Size.fromHeight(50),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
              ),
              onPressed: () async {
                if (_loginFormKey.currentState?.validate() ?? false) {
                  setState(() => _isLoading = true);
                  try {
                    await auth.login(
                      _loginController.text.trim(),
                      _loginPasswordController.text.trim(),
                      context,
                    );
                  } catch (e) {
                    _showSnack('Ошибка: $e');
                  } finally {
                    setState(() => _isLoading = false);
                  }
                }
              },
              child: Text('Login'.toUpperCase()),
            ),
            const SizedBox(height: 12),
            TextButton(
              onPressed: () {
                Navigator.pushNamed(context, '/my-resetpas');
              },
              child: Text(
                'Забыли пароль?',
                style: TextStyle(color: Theme.of(context).primaryColor),
              ),
            )
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Stack(
        children: [
          Container(color: Colors.white),
          Align(
            alignment: Alignment(0, 0.5),
            child: Container(
              width: 180,
              height: 180,
              decoration: const BoxDecoration(
                  shape: BoxShape.circle, color: Colors.white),
              child: Padding(
                padding: const EdgeInsets.all(15),
                child: Image.asset('assets/amanzat_logo.png'),
              ),
            ),
          ),
          Column(
            children: [
              const SizedBox(height: 80),
              Animate(
                effects: [FadeEffect(duration: 1.seconds)],
                child: Text(
                  'Добро пожаловать!',
                  style: TextStyle(
                      fontSize: 26,
                      fontWeight: FontWeight.bold,
                      color: Theme.of(context).primaryColor),
                ),
              ),
              const SizedBox(height: 20),
              Animate(
                effects: [
                  SlideEffect(
                      begin: Offset(0, -1),
                      end: Offset(0, 0),
                      duration: 1.seconds)
                ],
                child: TabBar(
                  controller: _tabController,
                  indicatorColor: Theme.of(context).primaryColor,
                  labelColor: Theme.of(context).primaryColor,
                  unselectedLabelColor:
                      Theme.of(context).primaryColor.withOpacity(0.6),
                  tabs: const [Tab(text: 'Регистрация'), Tab(text: 'Вход')],
                ),
              ),
              Expanded(
                child: TabBarView(
                  controller: _tabController,
                  children: [_buildRegisterTab(), _buildLoginTab()],
                ),
              ),
            ],
          ),
          if (_isLoading)
            Center(
              child: Lottie.asset('assets/animation/loading_animation.json',
                  width: 100, height: 100),
            ),
        ],
      ),
    );
  }
}
