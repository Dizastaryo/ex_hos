import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen>
    with TickerProviderStateMixin {
  late final AnimationController _logoController;
  late final Animation<double> _logoAnimation;
  late final AnimationController _textController;
  late final Animation<double> _textAnimation;

  @override
  void initState() {
    super.initState();

    // Инициализируем анимации
    _logoController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    );
    _logoAnimation =
        CurvedAnimation(parent: _logoController, curve: Curves.easeOutBack);

    _textController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
    _textAnimation =
        CurvedAnimation(parent: _textController, curve: Curves.easeIn);

    // Запускаем анимации последовательно
    _logoController.forward().then((_) => _textController.forward());

    // Сразу пытаемся автологин
    _handleAutoLogin();
  }

  Future<void> _handleAutoLogin() async {
    final auth = context.read<AuthProvider>();

    // 1) Проверяем, есть ли вообще сохранённый refreshToken
    final stored = await auth.loadRefreshTokenForDebug();
    if (stored == null) {
      // если токена нет — сразу на экран авторизации
      Navigator.pushReplacementNamed(context, '/auth');
      return;
    }

    // 2) Токен есть — ждём попытки silentAutoLogin()
    final success = await auth.tryRefreshToken();

    // 3) Навигация по результату
    if (success) {
      final route = auth.routeForCurrentUser();
      Navigator.pushReplacementNamed(context, route);
    } else {
      Navigator.pushReplacementNamed(context, '/auth');
    }
  }

  @override
  void dispose() {
    _logoController.dispose();
    _textController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF6A0DAD),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            ScaleTransition(
              scale: _logoAnimation,
              child: const Icon(
                Icons.shopping_bag_rounded,
                size: 100,
                color: Colors.white,
              ),
            ),
            const SizedBox(height: 24),
            FadeTransition(
              opacity: _textAnimation,
              child: const Text(
                'Aidyn Market',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 32,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 1.5,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
