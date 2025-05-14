import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

enum _Phase { animating, loading }

class _SplashScreenState extends State<SplashScreen>
    with TickerProviderStateMixin {
  late final AnimationController _logoController;
  late final Animation<double> _logoAnimation;
  late final AnimationController _textController;
  late final Animation<double> _textAnimation;

  _Phase _phase = _Phase.animating;

  @override
  void initState() {
    super.initState();

    // 1) Инициализируем контроллеры анимации
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

    // 2) Запускаем анимации последовательно
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _logoController.forward().then((_) {
        _textController.forward().then((_) {
          // 3) После анимаций переходим к фазе loading
          setState(() => _phase = _Phase.loading);
          _handleAutoLogin();
        });
      });
    });
  }

  Future<void> _handleAutoLogin() async {
    final auth = context.read<AuthProvider>();

    final stored = await auth.loadRefreshTokenForDebug();
    if (stored == null) {
      Navigator.pushReplacementNamed(context, '/auth');
      return;
    }

    final success = await auth.tryRefreshToken();
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

            // Если мы перешли в фазу loading — показываем спиннер
            if (_phase == _Phase.loading) ...[
              const SizedBox(height: 32),
              const CircularProgressIndicator(
                valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
              ),
              const SizedBox(height: 12),
              const Text(
                'Loading…',
                style: TextStyle(color: Colors.white),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
