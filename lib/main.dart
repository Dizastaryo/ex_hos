import 'dart:async';
import 'dart:math';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:workmanager/workmanager.dart';
import 'package:path_provider/path_provider.dart';
// Providers
import 'providers/auth_provider.dart';

// Services
import 'services/product_service.dart';
import 'services/category_service.dart';
import 'services/order_service.dart';

// Screens
import 'screens/product_detail_screen.dart';
import 'screens/payment_screen.dart';
import 'screens/payment_status_screen.dart';
import 'screens/home_screen.dart';
import 'screens/my_orders_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/support_screen.dart';
import 'screens/auth_screen.dart';
import 'screens/about_screen.dart';
import 'screens/my_cart_screen.dart';
import 'screens/notifications_screen.dart';
import 'screens/reset_password_screen.dart';
import 'screens/admin_home_screen.dart';
import 'screens/moderator_home_screen.dart';
import 'screens/products_screen.dart';
import 'screens/add_product_screen.dart';
import 'services/admin_service.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Инициализация уведомлений и фоновых задач
  await _initNotifications();
  await _requestNotificationPermissions();
  Workmanager().initialize(_callbackDispatcher);
  Workmanager().registerPeriodicTask(
    'notify_rentals',
    'notify_rentals',
    frequency: const Duration(days: 1),
  );

  // Настройка Dio и CookieJar
  final dio = Dio();
  final directory = await getApplicationDocumentsDirectory();
  final cookieJar = PersistCookieJar(
    storage: FileStorage("${directory.path}/.cookies/"),
  );
  dio.interceptors.add(CookieManager(cookieJar));

  // AuthProvider
  final authProvider = AuthProvider(dio, cookieJar);

  dio.interceptors.add(InterceptorsWrapper(
    onRequest: (opts, handler) async {
      // Access-token header
      if (authProvider.token != null) {
        opts.headers['Authorization'] = 'Bearer ${authProvider.token}';
      }
      return handler.next(opts);
    },
    onError: (err, handler) async {
      // on 401 - try refresh
      if (err.response?.statusCode == 401 &&
          !err.requestOptions.extra.containsKey('retry')) {
        try {
          await authProvider.refreshToken();
          err.requestOptions.extra['retry'] = true;
          return handler.resolve(await dio.fetch(err.requestOptions));
        } catch (_) {
          return handler.next(err);
        }
      }
      return handler.next(err);
    },
  ));

  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider<AuthProvider>.value(value: authProvider),
        Provider<ProductService>(create: (_) => ProductService(dio)),
        Provider<UserService>(create: (_) => UserService(dio)),
        Provider<CategoryService>(create: (_) => CategoryService(dio)),
        Provider<OrderService>(create: (_) => OrderService(dio)),
      ],
      child: const MyApp(),
    ),
  );
}

final FlutterLocalNotificationsPlugin _notificationsPlugin =
    FlutterLocalNotificationsPlugin();

Future<void> _initNotifications() async {
  const androidSettings = AndroidInitializationSettings('app_icon');
  const iosSettings = DarwinInitializationSettings();
  const settings = InitializationSettings(
    android: androidSettings,
    iOS: iosSettings,
  );
  await _notificationsPlugin.initialize(settings);
}

Future<void> _requestNotificationPermissions() async {
  if (!await Permission.notification.isGranted) {
    await Permission.notification.request();
  }
}

void _callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    if (task == 'notify_rentals') {
      // TODO: logic for notifications
      return Future.value(true);
    }
    return Future.value(false);
  });
}

class MyApp extends StatelessWidget {
  const MyApp({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Mag Service',
      theme: ThemeData(
        primaryColor: const Color(0xFF6C9942),
        colorScheme: const ColorScheme.light(
          primary: Color(0xFF6C9942),
          secondary: Color(0xFF4A6E2B),
        ),
        fontFamily: 'Montserrat',
        useMaterial3: true,
      ),
      initialRoute: '/splash',
      onGenerateRoute: (settings) {
        Widget page;
        switch (settings.name) {
          case '/splash':
            page = const SplashScreen();
            break;
          case '/auth':
            page = const AuthScreen();
            break;
          case '/main':
            page = const HomeScreen();
            break;
          case '/products':
            page = const ProductsScreen();
            break;
          case '/add-product':
            page = const AddProductScreen();
            break;
          case '/my-cart':
            page = const MyCartScreen();
            break;
          case '/notifications':
            page = const NotificationsScreen();
            break;
          case '/reset-password':
            page = const ResetPasswordScreen();
            break;
          case '/admin-home':
            page = const AdminHomeScreen();
            break;
          case '/moderator-home':
            page = const ModeratorHomeScreen();
            break;
          case '/support':
            page = SupportScreen();
            break;
          case '/about':
            page = const AboutScreen();
            break;
          case '/product-detail':
            final id = settings.arguments as int;
            page = ProductDetailScreen(productId: id);
            break;
          case '/orders':
            page = const MyOrdersScreen();
            break;
          case '/payment':
            final args = settings.arguments as Map<String, dynamic>;
            page = PaymentScreen(
              orderId: args['orderId'] as int,
              orderTotal: args['orderTotal'] as double,
            );
            break;
          case '/payment-status':
            final pid = settings.arguments as int;
            page = PaymentStatusScreen(orderId: pid);
            break;
          default:
            page = const SplashScreen();
        }
        return CircularRevealRoute(page: page);
      },
    );
  }
}

class CircularRevealRoute extends PageRouteBuilder {
  final Widget page;
  CircularRevealRoute({required this.page})
      : super(
          transitionDuration: const Duration(milliseconds: 700),
          pageBuilder: (context, animation, secondaryAnimation) => page,
          transitionsBuilder: (context, animation, secondaryAnimation, child) {
            return ClipOval(
              clipper: CircleRevealClipper(
                fraction: animation.value,
                centerOffset: Offset(
                  MediaQuery.of(context).size.width / 2,
                  MediaQuery.of(context).size.height / 2,
                ),
              ),
              child: child,
            );
          },
        );
}

class CircleRevealClipper extends CustomClipper<Rect> {
  final double fraction;
  final Offset centerOffset;

  CircleRevealClipper({required this.fraction, required this.centerOffset});

  @override
  Rect getClip(Size size) {
    final maxRadius = sqrt(size.width * size.width + size.height * size.height);
    final radius = maxRadius * fraction;
    return Rect.fromCircle(center: centerOffset, radius: radius);
  }

  @override
  bool shouldReclip(CircleRevealClipper old) {
    return fraction != old.fraction;
  }
}
