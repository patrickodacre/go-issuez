import './scss/index.scss';
import 'bootstrap';
const feather = require('feather-icons')
import resources from './resources'

window.onload = () => {

    /* globals Chart:false, feather:false */
    feather.replace()

    resources()
}
