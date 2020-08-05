import './scss/index.scss'
import 'bootstrap'
const feather = require('feather-icons')
import resources from './resources'
import projectPageModule from './projectPageModule'

window.onload = () => {

    /* globals Chart:false, feather:false */
    feather.replace()

    resources()

    projectPageModule()
}
